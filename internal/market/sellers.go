package market

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func sellerHome(c *gin.Context, db *mongo.Database) {
	id := c.Param("id")
	if len(id) == 0 || len(id) > 64 || !utf8.ValidString(id) {
		fail(c, 400, "invalid_input")
		return
	}
	limit, cursor, ok := parsePage(c, "", id)
	if !ok {
		return
	}
	var profile Profile
	err := db.Collection("users").FindOne(c.Request.Context(), bson.M{"_id": id}).Decode(&profile)
	if errors.Is(err, mongo.ErrNoDocuments) {
		fail(c, 404, "user_not_found")
		return
	}
	if err != nil {
		fail(c, 503, "unavailable")
		return
	}
	// Count branches before the cursor predicate: it is the entire eligible set,
	// not the remaining page. A single facet keeps count/list eligibility identical.
	page := mongo.Pipeline{}
	if cursor != nil {
		page = append(page, bson.D{{Key: "$match", Value: bson.M{"$or": bson.A{
			bson.M{"publishedAt": bson.M{"$lt": cursor.PublishedAt}},
			bson.M{"publishedAt": cursor.PublishedAt, "_id": bson.M{"$lt": cursor.ID}},
		}}}})
	}
	page = append(page, bson.D{{Key: "$limit", Value: int64(limit + 1)}}, bson.D{{Key: "$project", Value: bson.M{"eligibility": 0}}})
	pipeline := eligibleProducts(bson.M{"sellerId": id}, time.Now())
	pipeline = append(pipeline, bson.D{{Key: "$facet", Value: bson.M{
		"items": page, "counts": mongo.Pipeline{bson.D{{Key: "$count", Value: "total"}}},
	}}})
	rows, err := db.Collection("products").Aggregate(c.Request.Context(), pipeline)
	if err != nil {
		fail(c, 503, "unavailable")
		return
	}
	var result []struct {
		Items  []Product `bson:"items"`
		Counts []struct {
			Total int64 `bson:"total"`
		} `bson:"counts"`
	}
	if err := rows.All(c.Request.Context(), &result); err != nil || len(result) != 1 {
		fail(c, 503, "unavailable")
		return
	}
	items := result[0].Items
	if items == nil {
		items = []Product{}
	}
	count := int64(0)
	if len(result[0].Counts) != 0 {
		count = result[0].Counts[0].Total
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		data, _ := json.Marshal(browseCursor{SellerID: id, PublishedAt: last.PublishedAt, ID: last.ID})
		next = base64.RawURLEncoding.EncodeToString(data)
	}
	c.JSON(200, gin.H{"profile": profile, "creditScore": profile.CreditScore, "completedSales": profile.CompletedSales, "currentOnSaleCount": count, "items": items, "nextCursor": next})
}
