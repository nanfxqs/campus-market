package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func fixtureDB(t *testing.T) (*mongo.Client, *mongo.Database) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, db, err := connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) })
	return client, db
}
func postSale(t *testing.T, id, key string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"buyer": "buyer", "priceCents": 8500, "key": key})
	response, err := http.Post(os.Getenv("API_URL")+"/products/"+id+"/confirm", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Error(err)
		return 0
	}
	defer response.Body.Close()
	return response.StatusCode
}
func addProduct(t *testing.T, db *mongo.Database, id string, expires time.Time) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.Collection("products").InsertOne(ctx, product{ID: id, Title: "事务案例", Seller: "seller", PriceHistory: []int{10000, 9000}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("listings").InsertOne(ctx, bson.M{"_id": id, "expiresAt": expires}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Collection("products").DeleteOne(ctx, bson.M{"_id": id})
		db.Collection("listings").DeleteOne(ctx, bson.M{"_id": id})
		db.Collection("transactions").DeleteMany(ctx, bson.M{"product": id})
	})
}
func saleCount(t *testing.T, db *mongo.Database) int {
	t.Helper()
	var row struct {
		Count int `bson:"completedSales"`
	}
	if err := db.Collection("users").FindOne(context.Background(), bson.M{"_id": "seller"}).Decode(&row); err != nil {
		t.Fatal(err)
	}
	return row.Count
}
func assertPersistence(t *testing.T, db *mongo.Database, id string, sold bool, listings, records int64, count int) {
	t.Helper()
	ctx := context.Background()
	var p product
	if err := db.Collection("products").FindOne(ctx, bson.M{"_id": id}).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Sold != sold {
		t.Fatalf("sold=%v want %v", p.Sold, sold)
	}
	for _, v := range []struct {
		name   string
		filter bson.M
		want   int64
	}{{"listings", bson.M{"_id": id}, listings}, {"transactions", bson.M{"product": id, "success": true}, records}} {
		n, err := db.Collection(v.name).CountDocuments(ctx, v.filter)
		if err != nil || n != v.want {
			t.Fatalf("%s count=%d want=%d err=%v", v.name, n, v.want, err)
		}
	}
	if n := saleCount(t, db); n != count {
		t.Fatalf("seller count=%d want %d", n, count)
	}
}
func TestTransactionCommitAndRollback(t *testing.T) {
	_, db := fixtureDB(t)
	ctx := context.Background()
	initial := saleCount(t, db)
	addProduct(t, db, "commit-case", time.Now().Add(time.Hour))
	addProduct(t, db, "rollback-case", time.Now().Add(time.Hour))
	// A real collection validator rejects the successful-record write, after
	// product/listing writes. It is restored even if an assertion fails.
	command := bson.D{{Key: "collMod", Value: "transactions"}, {Key: "validator", Value: bson.M{"product": bson.M{"$ne": "rollback-case"}}}, {Key: "validationLevel", Value: "strict"}}
	if err := db.RunCommand(ctx, command).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "transactions"}, {Key: "validator", Value: bson.M{}}}).Err(); err != nil {
			t.Error(err)
		}
		db.Collection("users").UpdateOne(ctx, bson.M{"_id": "seller"}, bson.M{"$set": bson.M{"completedSales": initial}})
	})
	if status := postSale(t, "rollback-case", "rollback-key"); status != 409 {
		t.Fatalf("forced rollback status=%d", status)
	}
	assertPersistence(t, db, "rollback-case", false, 1, 0, initial)
	var wg sync.WaitGroup
	statuses := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); statuses <- postSale(t, "commit-case", fmt.Sprintf("commit-%d", i)) }(i)
	}
	wg.Wait()
	close(statuses)
	success := 0
	for s := range statuses {
		if s == 200 {
			success++
		} else if s != 409 {
			t.Fatalf("unexpected status %d", s)
		}
	}
	if success != 1 {
		t.Fatalf("successful competitors=%d want 1", success)
	}
	assertPersistence(t, db, "commit-case", true, 0, 1, initial+1)
}
func TestTTLRetainsArchiveAndForbidsSale(t *testing.T) {
	_, db := fixtureDB(t)
	ctx := context.Background()
	initial := saleCount(t, db)
	// Keep the expired fixture physically present during the HTTP assertion.
	// Restore the real zero-second TTL policy before observing storage cleanup.
	setTTL := func(seconds int) {
		t.Helper()
		if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "listings"}, {Key: "index", Value: bson.M{"name": "expiresAt_1", "expireAfterSeconds": seconds}}}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	setTTL(3600)
	t.Cleanup(func() {
		if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "listings"}, {Key: "index", Value: bson.M{"name": "expiresAt_1", "expireAfterSeconds": 0}}}).Err(); err != nil {
			t.Error(err)
		}
	})
	expires := time.Now().Add(5 * time.Second)
	addProduct(t, db, "ttl-case", expires)
	time.Sleep(time.Until(expires) + 20*time.Millisecond)
	assertPersistence(t, db, "ttl-case", false, 1, 0, initial)
	if status := postSale(t, "ttl-case", "expired-key"); status != 409 {
		t.Fatalf("expired sale status=%d", status)
	}
	assertPersistence(t, db, "ttl-case", false, 1, 0, initial)
	t.Log("expired listing remains physically present; HTTP rejects sale without writes before TTL cleanup")
	setTTL(0)
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		n, err := db.Collection("listings").CountDocuments(ctx, bson.M{"_id": "ttl-case"})
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			assertPersistence(t, db, "ttl-case", false, 0, 0, initial)
			var p product
			if err := db.Collection("products").FindOne(ctx, bson.M{"_id": "ttl-case"}).Decode(&p); err != nil || len(p.PriceHistory) != 2 {
				t.Fatalf("archive/history lost: %v", err)
			}
			t.Logf("TTL physical deletion observed %s after expiry; archive and history retained", time.Since(expires))
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("TTL observation timed out after 3m; inspect TTL index and server logs (not a business cleanup SLA)")
}

func TestScaleSearch(t *testing.T) {
	if os.Getenv("SCALE") != "1" {
		t.Skip("run after --large fixture with SCALE=1")
	}
	_, db := fixtureDB(t)
	ctx := context.Background()
	for _, v := range []struct {
		collection string
		want       int64
	}{{"users", 10000}, {"products", 200040}, {"transactions", 300000}} {
		n, err := db.Collection(v.collection).CountDocuments(ctx, bson.M{})
		if err != nil || n != v.want {
			t.Fatalf("%s=%d want %d err=%v", v.collection, n, v.want, err)
		}
	}
	for i := 0; i < 3; i++ {
		start := time.Now()
		resp, err := http.Get(os.Getenv("API_URL") + "/search?q=单车")
		if err != nil {
			t.Fatal(err)
		}
		var result searchResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || result.Total != 20000 || len(result.Results) != 20 {
			t.Fatalf("scale result: %+v err=%v", result, err)
		}
		t.Logf("scale search candidates=%d valid=%d top=%d server_ms=%d http=%s", result.Candidates, result.Total, len(result.Results), result.ElapsedMS, time.Since(start))
	}
}

func TestSearchChecksCurrentEligibility(t *testing.T) {
	_, db := fixtureDB(t)
	ctx := context.Background()
	expected := 27
	if os.Getenv("SCALE") == "1" {
		expected = 20000
	}
	for _, id := range []string{"p-000000", "p-000001", "p-000002"} {
		var listing bson.M
		if err := db.Collection("listings").FindOne(ctx, bson.M{"_id": id}).Decode(&listing); err != nil {
			t.Fatal(err)
		}
		savedID := id
		t.Cleanup(func() {
			db.Collection("listings").ReplaceOne(ctx, bson.M{"_id": savedID}, listing, options.Replace().SetUpsert(true))
			db.Collection("products").UpdateOne(ctx, bson.M{"_id": savedID}, bson.M{"$set": bson.M{"sold": false}})
		})
	}
	// These writes happen after index readiness. Search cannot use its stale
	// view of sold status or a fixed top-20 window as the current truth.
	if _, err := db.Collection("listings").UpdateOne(ctx, bson.M{"_id": "p-000000"}, bson.M{"$set": bson.M{"expiresAt": time.Now().Add(-time.Second)}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("products").UpdateOne(ctx, bson.M{"_id": "p-000001"}, bson.M{"$set": bson.M{"sold": true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("listings").DeleteOne(ctx, bson.M{"_id": "p-000002"}); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(os.Getenv("API_URL") + "/search?q=单车")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result searchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || result.Total != expected-3 || len(result.Results) != 20 {
		t.Fatalf("current eligibility total=%d results=%d status=%d", result.Total, len(result.Results), resp.StatusCode)
	}
	for _, r := range result.Results {
		if r.ID == "p-000000" || r.ID == "p-000001" || r.ID == "p-000002" {
			t.Fatalf("ineligible result %s", r.ID)
		}
	}
	t.Logf("current eligibility: %d candidates, %d eligible, 20 returned", result.Candidates, result.Total)
}
