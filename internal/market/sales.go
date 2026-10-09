package market

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type Sale struct {
	BuyerID     string    `bson:"buyerId" json:"buyerId"`
	PriceCents  int64     `bson:"priceCents" json:"priceCents"`
	ConfirmedAt time.Time `bson:"confirmedAt" json:"confirmedAt"`
}

type saleInput struct {
	BuyerID        string `json:"buyerId"`
	PriceCents     int64  `json:"priceCents"`
	IdempotencyKey string `json:"idempotencyKey"`
}

// SaleAttempt persists both committed sales and validated business failures.
// The seller/key pair owns one immutable result; HTTP retries reuse it.
type SaleAttempt struct {
	ID             string    `bson:"_id" json:"id"`
	SellerID       string    `bson:"sellerId" json:"sellerId"`
	ProductID      string    `bson:"productId" json:"productId"`
	BuyerID        string    `bson:"buyerId" json:"buyerId"`
	PriceCents     int64     `bson:"priceCents" json:"priceCents"`
	IdempotencyKey string    `bson:"idempotencyKey" json:"idempotencyKey"`
	Success        bool      `bson:"success" json:"success"`
	Code           string    `bson:"code" json:"code"`
	ConfirmedAt    time.Time `bson:"confirmedAt" json:"confirmedAt"`
}

func saleRoutes(authorized *gin.RouterGroup, db *mongo.Database, confirmationTime func() time.Time) {
	authorized.POST("/products/:id/sale", func(c *gin.Context) {
		var input saleInput
		if !decode(c, &input) {
			return
		}
		if input.PriceCents < 1 || input.PriceCents > 1000000000000 || input.BuyerID == "" || len(input.BuyerID) > 64 || input.BuyerID == c.GetString("userID") || !utf8.ValidString(input.IdempotencyKey) || len(input.IdempotencyKey) < 1 || len(input.IdempotencyKey) > 128 || strings.TrimSpace(input.IdempotencyKey) == "" {
			fail(c, 400, "invalid_input")
			return
		}
		var product Product
		if !loadProduct(c, db, &product) {
			return
		}
		if product.SellerID != c.GetString("userID") {
			fail(c, 403, "forbidden")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		if err := db.Collection("users").FindOne(ctx, bson.M{"_id": input.BuyerID}).Err(); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				fail(c, 400, "invalid_buyer")
			} else {
				fail(c, 503, "unavailable")
			}
			return
		}
		result, err := confirmSale(ctx, db, product, input, confirmationTime)
		if err != nil {
			log.Printf("sale transaction failed product=%q: %v", product.ID, err)
			fail(c, 503, "unavailable")
			return
		}
		if result.ProductID != product.ID || result.BuyerID != input.BuyerID || result.PriceCents != input.PriceCents {
			fail(c, 409, "idempotency_conflict")
			return
		}
		status := 200
		if !result.Success {
			status = 409
		}
		c.JSON(status, result)
	})
}

func confirmSale(ctx context.Context, db *mongo.Database, product Product, input saleInput, confirmationTime func() time.Time) (SaleAttempt, error) {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%s%s", len(product.SellerID), product.SellerID, input.IdempotencyKey)))
	id := hex.EncodeToString(digest[:])
	session, err := db.Client().StartSession()
	if err != nil {
		return SaleAttempt{}, err
	}
	defer session.EndSession(ctx)
	var result SaleAttempt
	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		priorErr := db.Collection("transactions").FindOne(sc, bson.M{"_id": id}).Decode(&result)
		if priorErr == nil {
			return nil, nil
		}
		if !errors.Is(priorErr, mongo.ErrNoDocuments) {
			return nil, priorErr
		}
		now := confirmationTime().UTC().Truncate(time.Millisecond)
		result = SaleAttempt{ID: id, SellerID: product.SellerID, ProductID: product.ID, BuyerID: input.BuyerID, PriceCents: input.PriceCents, IdempotencyKey: input.IdempotencyKey, Code: "sold", ConfirmedAt: now}
		var current Product
		if err := db.Collection("products").FindOne(sc, bson.M{"_id": product.ID, "sellerId": product.SellerID}).Decode(&current); err != nil {
			return nil, err
		}
		switch {
		case current.Sold:
			result.Code = "product_sold"
		case !current.ExpiresAt.After(now):
			result.Code = "product_expired"
		default:
			var listing struct {
				ExpiresAt time.Time `bson:"expiresAt"`
			}
			err := db.Collection("listings").FindOne(sc, bson.M{"_id": product.ID}).Decode(&listing)
			if errors.Is(err, mongo.ErrNoDocuments) {
				result.Code = "product_delisted"
			} else if err != nil {
				return nil, err
			} else if !listing.ExpiresAt.After(now) {
				result.Code = "product_expired"
			} else {
				// This write arbitrates competing confirmations; transaction retries re-read state.
				updated, err := db.Collection("products").UpdateOne(sc, bson.M{"_id": product.ID, "sellerId": product.SellerID, "sold": false, "expiresAt": bson.M{"$gt": now}}, bson.M{"$set": bson.M{"sold": true, "sale": Sale{BuyerID: input.BuyerID, PriceCents: input.PriceCents, ConfirmedAt: now}}})
				if err != nil {
					return nil, err
				}
				if updated.ModifiedCount != 1 {
					return nil, errors.New("sale condition changed")
				}
				removed, err := db.Collection("listings").DeleteOne(sc, bson.M{"_id": product.ID, "expiresAt": bson.M{"$gt": now}})
				if err != nil {
					return nil, err
				}
				if removed.DeletedCount != 1 {
					return nil, errors.New("sale eligibility changed")
				}
				updated, err = db.Collection("users").UpdateOne(sc, bson.M{"_id": product.SellerID}, bson.M{"$inc": bson.M{"completedSales": 1}})
				if err != nil {
					return nil, err
				}
				if updated.MatchedCount != 1 {
					return nil, errors.New("seller missing")
				}
				result.Success = true
			}
		}
		_, err := db.Collection("transactions").InsertOne(sc, result)
		return nil, err
	}, options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority()))
	if mongo.IsDuplicateKeyError(err) {
		// A competing request committed this seller/key. Our entire transaction rolled back.
		err = db.Collection("transactions", options.Collection().SetReadConcern(readconcern.Majority())).FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	}
	return result, err
}

// Runs before authentication so rejected credentials are logged too.
func saleRequestLog(c *gin.Context) {
	c.Next()
	if strings.HasSuffix(c.Request.URL.Path, "/sale") && c.Writer.Status() >= 400 {
		log.Printf("sale request rejected status=%d seller=%q product=%q", c.Writer.Status(), c.GetString("userID"), c.Param("id"))
	}
}
