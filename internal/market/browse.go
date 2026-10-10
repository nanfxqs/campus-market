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
	CategoryID  string    `json:"categoryId,omitempty"`
	SellerID    string    `json:"sellerId,omitempty"`
	PublishedAt time.Time `json:"publishedAt"`
	ID          string    `json:"id"`
}

// pageScope represents exactly one of the two supported browsing scopes.
// The cursor's wire fields remain separate for backwards compatibility.
type pageScope struct {
	seller bool
	id     string
}

func categoryPageScope(id string) pageScope { return pageScope{id: id} }
func sellerPageScope(id string) pageScope   { return pageScope{seller: true, id: id} }

func (s pageScope) cursor(last Product) browseCursor {
	value := browseCursor{PublishedAt: last.PublishedAt, ID: last.ID}
	if s.seller {
		value.SellerID = s.id
	} else {
		value.CategoryID = s.id
	}
	return value
}

func (s pageScope) matches(value browseCursor) bool {
	want := s.cursor(Product{})
	return value.CategoryID == want.CategoryID && value.SellerID == want.SellerID
}

func pagePosition(cursor *browseCursor) bson.M {
	if cursor == nil {
		return bson.M{}
	}
	return bson.M{"$or": bson.A{
		bson.M{"publishedAt": bson.M{"$lt": cursor.PublishedAt}},
		bson.M{"publishedAt": cursor.PublishedAt, "_id": bson.M{"$lt": cursor.ID}},
	}}
}

func finishPage(items []Product, limit int, scope pageScope) ([]Product, string) {
	if items == nil {
		items = []Product{}
	}
	if len(items) <= limit {
		return items, ""
	}
	items = items[:limit]
	data, _ := json.Marshal(scope.cursor(items[len(items)-1]))
	return items, base64.RawURLEncoding.EncodeToString(data)
}

func parsePage(c *gin.Context, scope pageScope) (int, *browseCursor, bool) {
	limit := 20
	if raw, exists := c.Request.URL.Query()["limit"]; exists {
		if len(raw) != 1 {
			fail(c, 400, "invalid_input")
			return 0, nil, false
		}
		n, err := strconv.Atoi(raw[0])
		if err != nil || n < 1 || n > 100 {
			fail(c, 400, "invalid_input")
			return 0, nil, false
		}
		limit = n
	}
	var cursor *browseCursor
	if values, exists := c.Request.URL.Query()["cursor"]; exists {
		if len(values) != 1 || len(values[0]) > 512 {
			fail(c, 400, "invalid_input")
			return 0, nil, false
		}
		data, err := base64.RawURLEncoding.Strict().DecodeString(values[0])
		value := browseCursor{}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err != nil || decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF || !scope.matches(value) || value.PublishedAt.IsZero() || !value.PublishedAt.Equal(value.PublishedAt.Truncate(time.Millisecond)) {
			fail(c, 400, "invalid_input")
			return 0, nil, false
		}
		objectID, err := primitive.ObjectIDFromHex(value.ID)
		if err != nil || objectID.Hex() != value.ID {
			fail(c, 400, "invalid_input")
			return 0, nil, false
		}
		cursor = &value
	}
	return limit, cursor, true
}

func browsePage(c *gin.Context, db *mongo.Database, clock func() time.Time) {
	category := c.Query("categoryId")
	if category == "" || len(category) > 64 || len(c.Request.URL.Query()["categoryId"]) != 1 {
		fail(c, 400, "invalid_input")
		return
	}
	scope := categoryPageScope(category)
	limit, cursor, ok := parsePage(c, scope)
	if !ok {
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
	filter := pagePosition(cursor)
	filter["categoryId"] = category
	pipeline := eligibleProducts(filter, clock())
	pipeline = append(pipeline, bson.D{{Key: "$limit", Value: int64(limit + 1)}}, bson.D{{Key: "$project", Value: bson.M{"eligibility": 0}}})
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
	items, next := finishPage(items, limit, scope)
	c.JSON(200, gin.H{"items": items, "nextCursor": next})
}

// Archive and listing deadlines both govern eligibility, independently of TTL.
func eligibleProducts(filter bson.M, now time.Time) mongo.Pipeline {
	filter["sold"] = false
	filter["expiresAt"] = bson.M{"$gt": now}
	return mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$sort", Value: bson.D{{Key: "publishedAt", Value: -1}, {Key: "_id", Value: -1}}}},
		{{Key: "$lookup", Value: bson.M{"from": "listings", "localField": "_id", "foreignField": "_id", "as": "eligibility"}}},
		{{Key: "$match", Value: bson.M{"eligibility": bson.M{"$elemMatch": bson.M{"expiresAt": bson.M{"$gt": now}}}}}},
	}
}
