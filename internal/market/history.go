package market

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type historyCursor struct {
	ChangedAt time.Time `json:"changedAt"`
	ID        string    `json:"id"`
}

func priceHistory(ctx context.Context, db *mongo.Database, id string, limit int, cursor *historyCursor) ([]PriceChange, error) {
	filter := bson.M{"productId": id}
	if cursor != nil {
		filter["$or"] = bson.A{
			bson.M{"changedAt": bson.M{"$lt": cursor.ChangedAt}},
			bson.M{"changedAt": cursor.ChangedAt, "_id": bson.M{"$lt": cursor.ID}},
		}
	}
	rows, err := db.Collection("priceChanges").Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "changedAt", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	items := []PriceChange{}
	err = rows.All(ctx, &items)
	return items, err
}
func historyPage(c *gin.Context, db *mongo.Database, id string) {
	limit := 20
	if raw, exists := c.GetQuery("limit"); exists {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			fail(c, 400, "invalid_input")
			return
		}
		limit = n
	}
	var cursor *historyCursor
	if raw, exists := c.GetQuery("cursor"); exists {
		data, err := base64.RawURLEncoding.DecodeString(raw)
		value := historyCursor{}
		if err != nil || len(raw) > 512 || json.Unmarshal(data, &value) != nil || value.ChangedAt.IsZero() {
			fail(c, 400, "invalid_input")
			return
		}
		if _, err := primitive.ObjectIDFromHex(value.ID); err != nil {
			fail(c, 400, "invalid_input")
			return
		}
		cursor = &value
	}
	items, err := priceHistory(c.Request.Context(), db, id, limit+1, cursor)
	if err != nil {
		fail(c, 503, "unavailable")
		return
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		data, _ := json.Marshal(historyCursor{ChangedAt: last.ChangedAt, ID: last.ID})
		next = base64.RawURLEncoding.EncodeToString(data)
	}
	c.JSON(200, gin.H{"items": items, "nextCursor": next})
}
