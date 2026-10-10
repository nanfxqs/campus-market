package market

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// initializeSearch installs the versioned synonym source and creates the Search
// index once. Index ingestion is asynchronous and does not block API startup.
func initializeSearch(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("synonyms_v1").UpdateOne(ctx, bson.M{"_id": "bicycles"}, bson.M{"$setOnInsert": bson.M{"mappingType": "equivalent", "synonyms": bson.A{"自行车", "单车", "脚踏车"}}}, options.Update().SetUpsert(true))
	if err != nil {
		return err
	}
	indexes, err := db.Collection("products").SearchIndexes().List(ctx, options.SearchIndexes().SetName("products_v1"))
	if err != nil {
		return err
	}
	defer indexes.Close(ctx)
	if indexes.Next(ctx) {
		return nil
	}
	if err := indexes.Err(); err != nil {
		return err
	}
	definition := bson.M{"mappings": bson.M{"dynamic": false, "fields": bson.M{"title": bson.M{"type": "string", "analyzer": "lucene.smartcn"}, "description": bson.M{"type": "string", "analyzer": "lucene.smartcn"}}}, "synonyms": bson.A{bson.M{"name": "bicycles_v1", "analyzer": "lucene.smartcn", "source": bson.M{"collection": "synonyms_v1"}}}}
	_, err = db.Collection("products").SearchIndexes().CreateOne(ctx, mongo.SearchIndexModel{Definition: definition, Options: options.SearchIndexes().SetName("products_v1")})
	return err
}

func searchPage(c *gin.Context, db *mongo.Database) {
	values := c.Request.URL.Query()["q"]
	if len(values) != 1 || !utf8.ValidString(values[0]) || strings.TrimSpace(values[0]) == "" || len(values[0]) > 300 {
		fail(c, 400, "invalid_input")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := search(ctx, db, values[0])
	if err != nil {
		fail(c, 503, "unavailable")
		return
	}
	c.JSON(200, result)
}

type searchHighlight struct {
	Path  string `bson:"path" json:"path"`
	Texts []struct {
		Value string `bson:"value" json:"value"`
		Type  string `bson:"type" json:"type"`
	} `bson:"texts" json:"texts"`
}
type searchHit struct {
	ID            string            `bson:"_id" json:"id"`
	Title         string            `bson:"title" json:"title"`
	Description   string            `bson:"description" json:"description"`
	Highlights    []searchHighlight `bson:"highlights" json:"highlights"`
	MatchedFields []string          `json:"matchedFields"`
}
type searchResult struct {
	Total   int         `json:"total"`
	Results []searchHit `json:"results"`
}

func search(ctx context.Context, db *mongo.Database, q string) (searchResult, error) {
	result := searchResult{Results: []searchHit{}}
	// No limit before eligibility checking: invalid high-ranking candidates must
	// neither inflate the total nor hide qualified candidates beyond the first 20.
	pipeline := mongo.Pipeline{
		{{Key: "$search", Value: bson.M{"index": "products_v1", "text": bson.M{"query": q, "path": bson.A{"title", "description"}, "synonyms": "bicycles_v1", "matchCriteria": "any"}, "highlight": bson.M{"path": bson.A{"title", "description"}}}}},
		{{Key: "$project", Value: bson.M{"title": 1, "description": 1, "highlights": bson.M{"$meta": "searchHighlights"}}}},
	}
	cursor, err := db.Collection("products").Aggregate(ctx, pipeline)
	if err != nil {
		return result, err
	}
	defer cursor.Close(ctx)
	// Batch database checks to bound memory and avoid one round trip per candidate.
	batch := make([]searchHit, 0, 500)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		ids := make([]string, 0, len(batch))
		for _, h := range batch {
			ids = append(ids, h.ID)
		}
		now := time.Now().UTC()
		cur, err := db.Collection("listings").Aggregate(ctx, mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"_id": bson.M{"$in": ids}, "expiresAt": bson.M{"$gt": now}}}},
			{{Key: "$lookup", Value: bson.M{"from": "products", "localField": "_id", "foreignField": "_id", "as": "product"}}},
			{{Key: "$match", Value: bson.M{"product": bson.M{"$elemMatch": bson.M{"sold": false, "expiresAt": bson.M{"$gt": now}}}}}},
			{{Key: "$project", Value: bson.M{"_id": 1}}},
		})
		if err != nil {
			return err
		}
		defer cur.Close(ctx)
		valid := map[string]bool{}
		for cur.Next(ctx) {
			var row struct {
				ID string `bson:"_id"`
			}
			if err := cur.Decode(&row); err != nil {
				return err
			}
			valid[row.ID] = true
		}
		if err := cur.Err(); err != nil {
			return err
		}
		for _, h := range batch {
			if !valid[h.ID] {
				continue
			}
			result.Total++
			if len(result.Results) < 20 {
				fields := map[string]bool{}
				for _, v := range h.Highlights {
					fields[v.Path] = true
				}
				for f := range fields {
					h.MatchedFields = append(h.MatchedFields, f)
				}
				sort.Strings(h.MatchedFields)
				result.Results = append(result.Results, h)
			}
		}
		batch = batch[:0]
		return nil
	}
	for cursor.Next(ctx) {
		var h searchHit
		if err := cursor.Decode(&h); err != nil {
			return result, err
		}
		batch = append(batch, h)
		if len(batch) == 500 {
			if err := flush(); err != nil {
				return result, err
			}
		}
	}
	if err := cursor.Err(); err != nil {
		return result, err
	}
	if err := flush(); err != nil {
		return result, err
	}
	return result, nil
}
