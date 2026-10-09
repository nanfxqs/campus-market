package market

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AttributeRule struct {
	Type     string `bson:"type" json:"type"`
	Required bool   `bson:"required" json:"required"`
}
type AttributeLimits struct {
	MaxAttributes      int `bson:"maxAttributes" json:"maxAttributes"`
	MaxExtraAttributes int `bson:"maxExtraAttributes" json:"maxExtraAttributes"`
	MaxKeyLength       int `bson:"maxKeyLength" json:"maxKeyLength"`
	MaxStringLength    int `bson:"maxStringLength" json:"maxStringLength"`
	MaxArrayLength     int `bson:"maxArrayLength" json:"maxArrayLength"`
}
type Category struct {
	ID     string                   `bson:"_id" json:"id"`
	Name   string                   `bson:"name" json:"name"`
	Rules  map[string]AttributeRule `bson:"rules" json:"rules"`
	Limits AttributeLimits          `bson:"limits" json:"limits"`
}

// Initialize installs platform categories without overwriting maintained rules.
// TTL removes only sale eligibility; product archives and history survive.
func Initialize(ctx context.Context, db *mongo.Database) error {
	categories := []Category{
		{ID: "textbooks", Name: "教材", Rules: map[string]AttributeRule{"author": {Type: "string", Required: true}, "isbn": {Type: "string"}, "edition": {Type: "number"}}},
		{ID: "bicycles", Name: "自行车", Rules: map[string]AttributeRule{"brand": {Type: "string", Required: true}, "wheelSize": {Type: "number", Required: true}, "foldable": {Type: "boolean"}}},
		{ID: "electronics", Name: "数码", Rules: map[string]AttributeRule{"brand": {Type: "string", Required: true}, "model": {Type: "string", Required: true}, "storageGB": {Type: "number"}, "accessories": {Type: "string[]"}}},
	}
	for _, category := range categories {
		category.Limits = AttributeLimits{MaxAttributes: 32, MaxExtraAttributes: 16, MaxKeyLength: 40, MaxStringLength: 256, MaxArrayLength: 16}
		if _, err := db.Collection("categories").UpdateOne(ctx, bson.M{"_id": category.ID}, bson.M{"$setOnInsert": category}, options.Update().SetUpsert(true)); err != nil {
			return err
		}
	}
	if _, err := db.Collection("listings").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)}); err != nil {
		return err
	}
	_, err := db.Collection("priceChanges").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "productId", Value: 1}, {Key: "changedAt", Value: -1}, {Key: "_id", Value: -1}}})
	return err
}

type ProductContent struct {
	CategoryID  string         `bson:"categoryId" json:"categoryId"`
	Title       string         `bson:"title" json:"title"`
	Description string         `bson:"description" json:"description"`
	Condition   string         `bson:"condition" json:"condition"`
	Images      []string       `bson:"images" json:"images"`
	Attributes  map[string]any `bson:"attributes" json:"attributes"`
}
type Product struct {
	ID                string `bson:"_id" json:"id"`
	ProductContent    `bson:",inline"`
	SellerID          string    `bson:"sellerId" json:"sellerId"`
	PriceCents        int64     `bson:"priceCents" json:"priceCents"`
	InitialPriceCents int64     `bson:"initialPriceCents" json:"initialPriceCents"`
	PublishedAt       time.Time `bson:"publishedAt" json:"publishedAt"`
	ExpiresAt         time.Time `bson:"expiresAt" json:"expiresAt"`
	Sold              bool      `bson:"sold" json:"sold"`
}
type PriceChange struct {
	ID            string    `bson:"_id" json:"id"`
	ProductID     string    `bson:"productId" json:"productId"`
	OldPriceCents int64     `bson:"oldPriceCents" json:"oldPriceCents"`
	NewPriceCents int64     `bson:"newPriceCents" json:"newPriceCents"`
	ChangedAt     time.Time `bson:"changedAt" json:"changedAt"`
}

func productRoutes(router *gin.Engine, authorized *gin.RouterGroup, db *mongo.Database) {
	router.GET("/categories", func(c *gin.Context) {
		cursor, err := db.Collection("categories").Find(c.Request.Context(), bson.M{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		items := []Category{}
		if err := cursor.All(c.Request.Context(), &items); err != nil {
			fail(c, 503, "unavailable")
			return
		}
		c.JSON(200, gin.H{"items": items})
	})
	authorized.POST("/products", func(c *gin.Context) {
		var input struct {
			ProductContent
			PriceCents int64 `json:"priceCents"`
		}
		if !decodeLimit(c, &input, 65536) {
			return
		}
		if input.PriceCents < 1 || input.PriceCents > 1000000000000 {
			fail(c, 400, "invalid_input")
			return
		}
		if !validateContent(c, db, input.ProductContent) {
			return
		}
		now := time.Now().UTC().Truncate(time.Millisecond)
		p := Product{ID: primitive.NewObjectID().Hex(), ProductContent: input.ProductContent, SellerID: c.GetString("userID"), PriceCents: input.PriceCents, InitialPriceCents: input.PriceCents, PublishedAt: now, ExpiresAt: now.Add(1440 * time.Hour)}
		session, err := db.Client().StartSession()
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		defer session.EndSession(c.Request.Context())
		_, err = session.WithTransaction(c.Request.Context(), func(sc mongo.SessionContext) (any, error) {
			if _, err := db.Collection("products").InsertOne(sc, p); err != nil {
				return nil, err
			}
			_, err := db.Collection("listings").InsertOne(sc, bson.M{"_id": p.ID, "expiresAt": p.ExpiresAt})
			return nil, err
		})
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		c.Header("Location", "/products/"+p.ID)
		c.JSON(http.StatusCreated, p)
	})
	authorized.PUT("/products/:id", func(c *gin.Context) {
		var current Product
		if !loadProduct(c, db, &current) {
			return
		}
		if current.SellerID != c.GetString("userID") {
			fail(c, 403, "forbidden")
			return
		}
		var input ProductContent
		if !decodeLimit(c, &input, 65536) || !validateContent(c, db, input) {
			return
		}
		session, err := db.Client().StartSession()
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		defer session.EndSession(c.Request.Context())
		var edited Product
		_, err = session.WithTransaction(c.Request.Context(), func(sc mongo.SessionContext) (any, error) {
			now := time.Now()
			// Read eligibility and update the archive in the same transaction.
			// Concurrent sale updates to this archive cause a transaction retry.
			if err := db.Collection("listings").FindOne(sc, bson.M{"_id": current.ID, "expiresAt": bson.M{"$gt": now}}).Err(); err != nil {
				return nil, err
			}
			err := db.Collection("products").FindOneAndUpdate(sc, bson.M{"_id": current.ID, "sellerId": c.GetString("userID"), "sold": false, "expiresAt": bson.M{"$gt": now}}, bson.M{"$set": input}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&edited)
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
		c.JSON(200, edited)
	})
	router.GET("/products/:id", func(c *gin.Context) {
		var p Product
		if !loadProduct(c, db, &p) {
			return
		}
		var seller Profile
		if err := db.Collection("users").FindOne(c.Request.Context(), bson.M{"_id": p.SellerID}).Decode(&seller); err != nil {
			fail(c, 503, "unavailable")
			return
		}
		count, err := db.Collection("listings").CountDocuments(c.Request.Context(), bson.M{"_id": p.ID, "expiresAt": bson.M{"$gt": time.Now()}})
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		history, err := priceHistory(c.Request.Context(), db, p.ID, 5, nil)
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		c.JSON(200, gin.H{"product": p, "seller": seller, "available": count == 1 && !p.Sold && p.ExpiresAt.After(time.Now()), "priceHistory": history, "priceHistoryUrl": "/products/" + p.ID + "/price-history"})
	})
	router.GET("/products/:id/price-history", func(c *gin.Context) {
		var p Product
		if !loadProduct(c, db, &p) {
			return
		}
		historyPage(c, db, p.ID)
	})
}
func loadProduct(c *gin.Context, db *mongo.Database, p *Product) bool {
	if _, err := primitive.ObjectIDFromHex(c.Param("id")); err != nil {
		fail(c, 400, "invalid_input")
		return false
	}
	err := db.Collection("products").FindOne(c.Request.Context(), bson.M{"_id": c.Param("id")}).Decode(p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		fail(c, 404, "product_not_found")
		return false
	}
	if err != nil {
		fail(c, 503, "unavailable")
		return false
	}
	return true
}
