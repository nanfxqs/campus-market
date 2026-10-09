package market

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type browseCursor struct {
	CategoryID  string    `json:"categoryId"`
	PublishedAt time.Time `json:"publishedAt"`
	ID          string    `json:"id"`
}

func browsePage(c *gin.Context, db *mongo.Database) {
	category := c.Query("categoryId")
	if category == "" || len(category) > 64 {
		fail(c, 400, "invalid_input")
		return
	}
	limit := 20
	if raw, exists := c.Request.URL.Query()["limit"]; exists {
		if len(raw) != 1 {
			fail(c, 400, "invalid_input")
			return
		}
		n, err := strconv.Atoi(raw[0])
		if err != nil || n < 1 || n > 100 {
			fail(c, 400, "invalid_input")
			return
		}
		limit = n
	}
	if len(c.Request.URL.Query()["categoryId"]) != 1 {
		fail(c, 400, "invalid_input")
		return
	}
	var cursor *browseCursor
	if values, exists := c.Request.URL.Query()["cursor"]; exists {
		if len(values) != 1 || len(values[0]) > 512 {
			fail(c, 400, "invalid_input")
			return
		}
		data, err := base64.RawURLEncoding.Strict().DecodeString(values[0])
		value := browseCursor{}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err != nil || decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF || value.CategoryID != category || value.PublishedAt.IsZero() || !value.PublishedAt.Equal(value.PublishedAt.Truncate(time.Millisecond)) {
			fail(c, 400, "invalid_input")
			return
		}
		objectID, err := primitive.ObjectIDFromHex(value.ID)
		if err != nil || objectID.Hex() != value.ID {
			fail(c, 400, "invalid_input")
			return
		}
		cursor = &value
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
	filter := bson.M{"categoryId": category, "sold": false, "expiresAt": bson.M{"$gt": now}}
	if cursor != nil {
		filter["$or"] = bson.A{bson.M{"publishedAt": bson.M{"$lt": cursor.PublishedAt}}, bson.M{"publishedAt": cursor.PublishedAt, "_id": bson.M{"$lt": cursor.ID}}}
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$sort", Value: bson.D{{Key: "publishedAt", Value: -1}, {Key: "_id", Value: -1}}}},
		{{Key: "$lookup", Value: bson.M{"from": "listings", "localField": "_id", "foreignField": "_id", "as": "eligibility"}}},
		{{Key: "$match", Value: bson.M{"eligibility": bson.M{"$elemMatch": bson.M{"expiresAt": bson.M{"$gt": now}}}}}},
		{{Key: "$limit", Value: int64(limit + 1)}},
		{{Key: "$project", Value: bson.M{"eligibility": 0}}},
	}
	rows, err := db.Collection("products").Aggregate(c.Request.Context(), pipeline)
	if err != nil {
		fail(c, 503, "unavailable")
		return
	}
	items := []Product{}
	if err := rows.All(c.Request.Context(), &items); err != nil {
		fail(c, 503, "unavailable")
		return
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		data, _ := json.Marshal(browseCursor{CategoryID: category, PublishedAt: last.PublishedAt, ID: last.ID})
		next = base64.RawURLEncoding.EncodeToString(data)
	}
	c.JSON(200, gin.H{"items": items, "nextCursor": next})
}
