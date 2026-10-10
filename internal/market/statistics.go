package market

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func statisticsRoutes(router *gin.Engine, db *mongo.Database) {
	router.GET("/categories/:id/condition-counts", func(c *gin.Context) {
		category := c.Param("id")
		if len(category) > 64 {
			fail(c, 400, "invalid_input")
			return
		}
		err := db.Collection("categories").FindOne(c.Request.Context(), bson.M{"_id": category}).Err()
		if errors.Is(err, mongo.ErrNoDocuments) {
			fail(c, 404, "category_not_found")
			return
		}
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		now := time.Now()
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"categoryId": category, "sold": false, "expiresAt": bson.M{"$gt": now}}}},
			{{Key: "$lookup", Value: bson.M{"from": "listings", "localField": "_id", "foreignField": "_id", "as": "eligibility"}}},
			{{Key: "$match", Value: bson.M{"eligibility": bson.M{"$elemMatch": bson.M{"expiresAt": bson.M{"$gt": now}}}}}},
			{{Key: "$group", Value: bson.M{"_id": "$condition", "count": bson.M{"$sum": 1}}}},
		}
		cursor, err := db.Collection("products").Aggregate(c.Request.Context(), pipeline)
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		var rows []struct {
			Condition string `bson:"_id"`
			Count     int64  `bson:"count"`
		}
		if err := cursor.All(c.Request.Context(), &rows); err != nil {
			fail(c, 503, "unavailable")
			return
		}
		counts := map[string]int64{}
		for _, row := range rows {
			counts[row.Condition] = row.Count
		}
		items := []gin.H{}
		for _, condition := range productConditions {
			items = append(items, gin.H{"condition": condition, "count": counts[condition]})
		}
		c.JSON(200, gin.H{"categoryId": category, "items": items})
	})
}
