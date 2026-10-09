// Command validation is an isolated feasibility harness, not the V1 application.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type product struct {
	ID           string `bson:"_id" json:"id"`
	Title        string `bson:"title" json:"title"`
	Description  string `bson:"description" json:"description"`
	Seller       string `bson:"seller" json:"seller"`
	Sold         bool   `bson:"sold" json:"sold"`
	PriceHistory []int  `bson:"priceHistory" json:"priceHistory"`
}

func env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}
func connect(ctx context.Context) (*mongo.Client, *mongo.Database, error) {
	name := env("DB_NAME", "campus_validation")
	if name != "campus_validation" {
		return nil, nil, errors.New("harness restricted to campus_validation database")
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(env("MONGO_URI", "mongodb://localhost:27017/?directConnection=true")))
	if err != nil {
		return nil, nil, err
	}
	if err = client.Ping(ctx, nil); err != nil {
		return nil, nil, err
	}
	return client, client.Database(name), nil
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "health" {
		resp, err := http.Get("http://localhost:8080/health")
		if err != nil {
			os.Exit(1)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client, db, err := connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "seed":
		large, reset := false, false
		for _, arg := range os.Args[2:] {
			switch arg {
			case "--large":
				large = true
			case "--reset":
				reset = true
			default:
				log.Fatalf("unknown argument %s", arg)
			}
		}
		if err := seed(ctx, db, large, reset); err != nil {
			log.Fatal(err)
		}
	case "serve":
		r := gin.New()
		r.Use(gin.Recovery())
		r.GET("/health", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
			defer cancel()
			if err := client.Ping(ctx, nil); err != nil {
				c.JSON(503, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"ready": true})
		})
		r.GET("/search", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
			defer cancel()
			q := c.Query("q")
			if q == "" || len(q) > 300 {
				c.JSON(400, gin.H{"error": "q must contain 1–300 bytes"})
				return
			}
			result, err := search(ctx, db, q)
			if err != nil {
				c.JSON(503, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, result)
		})
		r.POST("/products/:id/confirm", func(c *gin.Context) {
			var input struct {
				Buyer string `json:"buyer"`
				Price int    `json:"priceCents"`
				Key   string `json:"key"`
			}
			if c.ShouldBindJSON(&input) != nil || input.Buyer != "buyer" || input.Price <= 0 || input.Key == "" {
				c.JSON(400, gin.H{"error": "fixture buyer, positive priceCents and key required"})
				return
			}
			err := confirm(c.Request.Context(), client, db, c.Param("id"), input.Buyer, input.Price, input.Key)
			if err != nil {
				c.JSON(409, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"confirmed": true})
		})
		log.Fatal(r.Run(":8080"))
	default:
		log.Fatalf("unknown command %s", command)
	}
}

type highlight struct {
	Path  string `bson:"path" json:"path"`
	Texts []struct {
		Value string `bson:"value" json:"value"`
		Type  string `bson:"type" json:"type"`
	} `bson:"texts" json:"texts"`
}
type hit struct {
	ID            string      `bson:"_id" json:"id"`
	Title         string      `bson:"title" json:"title"`
	Description   string      `bson:"description" json:"description"`
	Highlights    []highlight `bson:"highlights" json:"highlights"`
	MatchedFields []string    `json:"matchedFields"`
}
type searchResult struct {
	Total      int   `json:"total"`
	Results    []hit `json:"results"`
	Candidates int   `json:"candidates"`
	ElapsedMS  int64 `json:"elapsedMs"`
}

func search(ctx context.Context, db *mongo.Database, q string) (searchResult, error) {
	start := time.Now()
	result := searchResult{Results: []hit{}}
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
	batch := make([]hit, 0, 500)
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
			{{Key: "$match", Value: bson.M{"product.sold": false}}},
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
		var h hit
		if err := cursor.Decode(&h); err != nil {
			return result, err
		}
		result.Candidates++
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
	result.ElapsedMS = time.Since(start).Milliseconds()
	return result, nil
}

var unavailable = errors.New("product is sold, expired or unavailable")

func confirm(ctx context.Context, client *mongo.Client, db *mongo.Database, id, buyer string, price int, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	session, err := client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		var prior struct {
			Product string `bson:"product"`
			Buyer   string `bson:"buyer"`
			Price   int    `bson:"priceCents"`
		}
		err := db.Collection("transactions").FindOne(sc, bson.M{"_id": "seller:" + key}).Decode(&prior)
		if err == nil {
			if prior.Product == id && prior.Buyer == buyer && prior.Price == price {
				return nil, nil
			}
			return nil, errors.New("key reused with different content")
		}
		if err != mongo.ErrNoDocuments {
			return nil, err
		}
		now := time.Now().UTC()
		removed, err := db.Collection("listings").DeleteOne(sc, bson.M{"_id": id, "expiresAt": bson.M{"$gt": now}})
		if err != nil {
			return nil, err
		}
		if removed.DeletedCount != 1 {
			return nil, unavailable
		}
		updated, err := db.Collection("products").UpdateOne(sc, bson.M{"_id": id, "seller": "seller", "sold": false}, bson.M{"$set": bson.M{"sold": true, "buyer": buyer, "salePriceCents": price, "confirmedAt": now}})
		if err != nil {
			return nil, err
		}
		if updated.ModifiedCount != 1 {
			return nil, unavailable
		}
		_, err = db.Collection("transactions").InsertOne(sc, bson.M{"_id": "seller:" + key, "product": id, "seller": "seller", "buyer": buyer, "priceCents": price, "success": true, "confirmedAt": now})
		if err != nil {
			return nil, err
		}
		count, err := db.Collection("users").UpdateOne(sc, bson.M{"_id": "seller"}, bson.M{"$inc": bson.M{"completedSales": 1}})
		if err != nil {
			return nil, err
		}
		if count.MatchedCount != 1 {
			return nil, errors.New("seller missing")
		}
		return nil, nil
	})
	return err
}

func seed(ctx context.Context, db *mongo.Database, large, reset bool) error {
	names, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return err
	}
	if len(names) > 0 && !reset {
		return errors.New("database is not empty; explicit --reset required")
	}
	if reset {
		if err := db.Drop(ctx); err != nil {
			return err
		}
	}
	if err := db.CreateCollection(ctx, "transactions"); err != nil {
		return err
	}
	now := time.Now().UTC()
	if s := os.Getenv("FIXTURE_TIME"); s != "" {
		now, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return err
		}
	}
	users, active, history, transactions := 2, 27, 0, 0
	if large {
		users, active, history, transactions = 10000, 20000, 180000, 300000
	}
	docs := make([]interface{}, 0, 1000)
	flush := func(collection string) error {
		if len(docs) == 0 {
			return nil
		}
		_, err := db.Collection(collection).InsertMany(ctx, docs)
		docs = docs[:0]
		return err
	}
	for i := 0; i < users; i++ {
		id := fmt.Sprintf("user-%06d", i)
		if i == 0 {
			id = "seller"
		}
		if i == 1 {
			id = "buyer"
		}
		count := 0
		if i == 0 {
			count = history
		}
		docs = append(docs, bson.M{"_id": id, "completedSales": count})
		if len(docs) == 1000 {
			if err := flush("users"); err != nil {
				return err
			}
		}
	}
	if err := flush("users"); err != nil {
		return err
	}
	terms := []string{"自行车", "单车", "脚踏车"}
	for i := 0; i < active+history+40; i++ {
		id := fmt.Sprintf("p-%06d", i)
		title, description := "教材", "课程资料"
		sold := i >= active && i < active+history
		if i < active || i >= active+history {
			term := terms[i%3]
			switch i % 3 {
			case 0:
				title = term
			case 1:
				description = term
			case 2:
				title = term
				description = term
			}
		}
		// Invalid candidates deliberately score above valid candidates.
		if i >= active+history {
			title = "自行车 单车 脚踏车"
			description = title
			sold = i%2 == 0
		}
		docs = append(docs, product{ID: id, Title: title, Description: description, Seller: "seller", Sold: sold, PriceHistory: []int{10000, 9000}})
		if len(docs) == 1000 {
			if err := flush("products"); err != nil {
				return err
			}
		}
	}
	if err := flush("products"); err != nil {
		return err
	}
	for i := 0; i < active; i++ {
		docs = append(docs, bson.M{"_id": fmt.Sprintf("p-%06d", i), "expiresAt": now.Add(60 * 24 * time.Hour)})
		if len(docs) == 1000 {
			if err := flush("listings"); err != nil {
				return err
			}
		}
	}
	if err := flush("listings"); err != nil {
		return err
	}
	// Extra expired and sold entries expose stale search candidates and TTL lag.
	for i := active + history; i < active+history+40; i++ {
		expiry := now.Add(-time.Hour)
		if i%2 == 0 {
			expiry = now.Add(time.Hour)
		}
		docs = append(docs, bson.M{"_id": fmt.Sprintf("p-%06d", i), "expiresAt": expiry})
	}
	if err := flush("listings"); err != nil {
		return err
	}
	for i := 0; i < transactions; i++ {
		success := i < history
		pid := fmt.Sprintf("p-%06d", active+i%max(history, 1))
		docs = append(docs, bson.M{"_id": fmt.Sprintf("history-%06d", i), "product": pid, "seller": "seller", "buyer": "buyer", "success": success, "priceCents": 8000})
		if len(docs) == 1000 {
			if err := flush("transactions"); err != nil {
				return err
			}
		}
	}
	if err := flush("transactions"); err != nil {
		return err
	}
	_, err = db.Collection("listings").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)})
	if err != nil {
		return err
	}
	_, err = db.Collection("synonyms_v1").InsertOne(ctx, bson.M{"mappingType": "equivalent", "synonyms": terms})
	if err != nil {
		return err
	}
	definition := bson.M{"mappings": bson.M{"dynamic": false, "fields": bson.M{"title": bson.M{"type": "string", "analyzer": "lucene.smartcn"}, "description": bson.M{"type": "string", "analyzer": "lucene.smartcn"}}}, "synonyms": bson.A{bson.M{"name": "bicycles_v1", "analyzer": "lucene.smartcn", "source": bson.M{"collection": "synonyms_v1"}}}}
	_, err = db.Collection("products").SearchIndexes().CreateOne(ctx, mongo.SearchIndexModel{Definition: definition, Options: options.SearchIndexes().SetName("products_v1")})
	if err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		result, e := search(ctx, db, "单车")
		if e == nil && result.Total == active {
			log.Printf("fixture ready users=%d active=%d history=%d transactions=%d candidates=%d search_ms=%d base=%s", users, active, history, transactions, result.Candidates, result.ElapsedMS, now.Format(time.RFC3339Nano))
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("Search readiness timed out: inspect products_v1 and synonyms_v1")
}
