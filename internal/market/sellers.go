package market

import (
	"errors"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func sellerHome(c *gin.Context, db *mongo.Database, clock func() time.Time) {
	id := c.Param("id")
	if len(id) == 0 || len(id) > 64 || !utf8.ValidString(id) {
		fail(c, 400, "invalid_input")
		return
	}
	scope := sellerPageScope(id)
	limit, cursor, ok := parsePage(c, scope)
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
		page = append(page, bson.D{{Key: "$match", Value: pagePosition(cursor)}})
	}
	page = append(page, bson.D{{Key: "$limit", Value: int64(limit + 1)}}, bson.D{{Key: "$project", Value: bson.M{"eligibility": 0}}})
	pipeline := eligibleProducts(bson.M{"sellerId": id}, clock())
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
	items, next := finishPage(result[0].Items, limit, scope)
	count := int64(0)
	if len(result[0].Counts) != 0 {
		count = result[0].Counts[0].Total
	}
	c.JSON(200, gin.H{"profile": profile, "creditScore": profile.CreditScore, "completedSales": profile.CompletedSales, "currentOnSaleCount": count, "items": items, "nextCursor": next})
}
