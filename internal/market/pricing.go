package market

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func priceRoutes(authorized *gin.RouterGroup, db *mongo.Database, clock func() time.Time) {
	authorized.PATCH("/products/:id/price", func(c *gin.Context) {
		var product Product
		if !loadProduct(c, db, &product) {
			return
		}
		if product.SellerID != c.GetString("userID") {
			fail(c, 403, "forbidden")
			return
		}
		var input struct {
			PriceCents int64 `json:"priceCents"`
		}
		if !decode(c, &input) {
			return
		}
		if input.PriceCents < 1 || input.PriceCents > 1000000000000 {
			fail(c, 400, "invalid_input")
			return
		}
		session, err := db.Client().StartSession()
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		defer session.EndSession(c.Request.Context())
		var updated Product
		_, err = session.WithTransaction(c.Request.Context(), func(sc mongo.SessionContext) (any, error) {
			now := clock().UTC().Truncate(time.Millisecond)
			if err := db.Collection("listings").FindOne(sc, bson.M{"_id": product.ID, "expiresAt": bson.M{"$gt": now}}).Err(); err != nil {
				return nil, err
			}
			// Updating the archive arbitrates concurrent price changes and sales.
			// Return its prior price so the history reflects the committed transition.
			var prior Product
			err := db.Collection("products").FindOneAndUpdate(sc, bson.M{"_id": product.ID, "sellerId": c.GetString("userID"), "sold": false, "expiresAt": bson.M{"$gt": now}}, bson.M{"$set": bson.M{"priceCents": input.PriceCents}}, options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&prior)
			if err != nil {
				return nil, err
			}
			updated = prior
			updated.PriceCents = input.PriceCents
			if prior.PriceCents == input.PriceCents {
				return nil, nil
			}
			_, err = db.Collection("priceChanges").InsertOne(sc, PriceChange{ID: primitive.NewObjectID().Hex(), ProductID: prior.ID, OldPriceCents: prior.PriceCents, NewPriceCents: input.PriceCents, ChangedAt: now})
			return nil, err
		})
		if errors.Is(err, mongo.ErrNoDocuments) {
			fail(c, 409, "product_unavailable")
			return
		}
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		c.JSON(200, updated)
	})
}
