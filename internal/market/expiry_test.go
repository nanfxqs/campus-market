package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func expiryDatabase(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Fatal("MONGO_URI required: use scripts/validate-expiry.sh")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database("campus_expiry_" + primitive.NewObjectID().Hex())
	t.Cleanup(func() { db.Drop(context.Background()); client.Disconnect(context.Background()) })
	if err := Seed(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db
}

func expiryRequest(t *testing.T, base, method, path, token, body string, want int) map[string]any {
	t.Helper()
	req, err := http.NewRequest(method, base+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s: status=%d want=%d body=%v", method, path, resp.StatusCode, want, result)
	}
	return result
}

func expiryProduct(t *testing.T, base, token string) string {
	t.Helper()
	result := expiryRequest(t, base, "POST", "/products", token, `{"categoryId":"bicycles","title":"自行车","description":"校园通勤","condition":"全新","images":["https://example.com/bike.png"],"attributes":{"brand":"校园","wheelSize":26},"priceCents":10000}`, 201)
	id := result["id"].(string)
	expiryRequest(t, base, "PATCH", "/products/"+id+"/price", token, `{"priceCents":9000}`, 200)
	return id
}

func setExpiry(t *testing.T, db *mongo.Database, id string, expires time.Time) {
	t.Helper()
	for _, name := range []string{"products", "listings"} {
		changes := bson.M{"expiresAt": expires}
		if name == "products" {
			changes["publishedAt"] = expires.Add(-1440 * time.Hour)
		}
		if _, err := db.Collection(name).UpdateOne(context.Background(), bson.M{"_id": id}, bson.M{"$set": changes}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExpiryLifecycleHTTP(t *testing.T) {
	db := expiryDatabase(t)
	ctx := context.Background()
	// Delay physical cleanup only in this isolated database; keep the real TTL index.
	if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "listings"}, {Key: "index", Value: bson.M{"name": "expiresAt_1", "expireAfterSeconds": 3600}}}).Err(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Millisecond)
	now := deadline.Add(-time.Millisecond)
	server := httptest.NewServer(newAPI(db, time.Hour, func() time.Time { return now }))
	defer server.Close()
	token := expiryRequest(t, server.URL, "POST", "/auth/login", "", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	id := expiryProduct(t, server.URL, token)
	past := expiryProduct(t, server.URL, token)
	setExpiry(t, db, id, deadline)
	setExpiry(t, db, past, deadline.Add(-time.Minute))
	// Wait for actual Search ingestion while the logical clock remains before expiry.
	ready := time.Now().Add(3 * time.Minute)
	for {
		result := expiryRequest(t, server.URL, "GET", "/search?q=单车", "", "", 200)
		if result["total"] == float64(1) {
			break
		}
		if time.Now().After(ready) {
			t.Fatalf("Search readiness timeout: %v", result)
		}
		time.Sleep(500 * time.Millisecond)
	}
	for _, offset := range []time.Duration{-time.Millisecond, 0, time.Millisecond} {
		t.Run(offset.String(), func(t *testing.T) {
			now = deadline.Add(offset)
			want := 0
			if offset < 0 {
				want = 1
			}
			browse := expiryRequest(t, server.URL, "GET", "/products?categoryId=bicycles", "", "", 200)
			home := expiryRequest(t, server.URL, "GET", "/users/seller/home", "", "", 200)
			search := expiryRequest(t, server.URL, "GET", "/search?q=单车", "", "", 200)
			counts := expiryRequest(t, server.URL, "GET", "/categories/bicycles/condition-counts", "", "", 200)
			detail := expiryRequest(t, server.URL, "GET", "/products/"+id, "", "", 200)
			if len(browse["items"].([]any)) != want || len(home["items"].([]any)) != want || home["currentOnSaleCount"] != float64(want) || search["total"] != float64(want) || len(search["results"].([]any)) != want || counts["items"].([]any)[0].(map[string]any)["count"] != float64(want) || detail["available"] != (want == 1) {
				t.Fatalf("inconsistent expiry: browse=%v home=%v search=%v counts=%v detail=%v", browse, home, search, counts, detail)
			}
			status := 409
			if want == 1 {
				status = 200
			}
			expiryRequest(t, server.URL, "PATCH", "/products/"+id+"/price", token, `{"priceCents":8000}`, status)
			if want == 0 {
				content := map[string]any{"categoryId": "bicycles", "title": "自行车", "description": "校园通勤", "condition": "全新", "images": []string{"https://example.com/bike.png"}, "attributes": map[string]any{"brand": "校园", "wheelSize": 26}}
				encoded, err := json.Marshal(content)
				if err != nil {
					t.Fatal(err)
				}
				expiryRequest(t, server.URL, "PUT", "/products/"+id, token, string(encoded), 409)
				expiryRequest(t, server.URL, "POST", "/products/"+id+"/sale", token, fmt.Sprintf(`{"buyerId":"buyer","priceCents":7000,"idempotencyKey":%q}`, offset.String()), 409)
			}
			t.Logf("all entry points expiry=%s now=%s eligible=%d", deadline.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), want)
		})
	}
	n, err := db.Collection("listings").CountDocuments(ctx, bson.M{"_id": bson.M{"$in": []string{id, past}}})
	if err != nil || n != 2 {
		t.Fatalf("TTL lag fixture missing: count=%d err=%v", n, err)
	}
	n, err = db.Collection("transactions").CountDocuments(ctx, bson.M{"success": true})
	if err != nil || n != 0 {
		t.Fatalf("expired success=%d err=%v", n, err)
	}
}

func TestExpiryTTLRetainsHistoryHTTP(t *testing.T) {
	db := expiryDatabase(t)
	ctx := context.Background()
	server := httptest.NewServer(New(db, time.Hour))
	defer server.Close()
	token := expiryRequest(t, server.URL, "POST", "/auth/login", "", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	id := expiryProduct(t, server.URL, token)
	before := expiryRequest(t, server.URL, "GET", "/products/"+id, "", "", 200)
	history := expiryRequest(t, server.URL, "GET", "/products/"+id+"/price-history", "", "", 200)
	expires := time.Now().UTC().Add(2 * time.Second).Truncate(time.Millisecond)
	setExpiry(t, db, id, expires)
	time.Sleep(time.Until(expires) + time.Millisecond)
	expiryRequest(t, server.URL, "PATCH", "/products/"+id+"/price", token, `{"priceCents":8000}`, 409)
	expiryRequest(t, server.URL, "POST", "/products/"+id+"/sale", token, `{"buyerId":"buyer","priceCents":7000,"idempotencyKey":"expired"}`, 409)
	deadline := time.Now().Add(3 * time.Minute)
	for {
		n, err := db.Collection("listings").CountDocuments(ctx, bson.M{"_id": id})
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			indexes, e := db.Collection("listings").Indexes().List(ctx)
			var rows []bson.M
			if e == nil {
				e = indexes.All(ctx, &rows)
			}
			t.Fatalf("TTL observation timed out (not a business SLA): db=%s id=%s expires=%s indexes=%v err=%v; inspect MongoDB TTL monitor and logs", db.Name(), id, expires, rows, e)
		}
		time.Sleep(500 * time.Millisecond)
	}
	after := expiryRequest(t, server.URL, "GET", "/products/"+id, "", "", 200)
	p := after["product"].(map[string]any)
	original := before["product"].(map[string]any)
	for _, key := range []string{"id", "title", "description", "images", "attributes", "priceCents", "initialPriceCents"} {
		if !reflect.DeepEqual(p[key], original[key]) {
			t.Fatalf("archive changed %s: %v", key, after)
		}
	}
	if after["available"] != false || !reflect.DeepEqual(after["priceHistory"], before["priceHistory"]) || !reflect.DeepEqual(history, expiryRequest(t, server.URL, "GET", "/products/"+id+"/price-history", "", "", 200)) {
		t.Fatalf("archive/history lost: %v", after)
	}
	expiryRequest(t, server.URL, "PATCH", "/products/"+id+"/price", token, `{"priceCents":8000}`, 409)
	expiryRequest(t, server.URL, "POST", "/products/"+id+"/sale", token, `{"buyerId":"buyer","priceCents":7000,"idempotencyKey":"after-ttl"}`, 409)
	t.Logf("TTL database evidence db=%s listing=%s count=0 expiresAt=%s observed=%s cleanup_delay=%s; archive, images, price history readable; sale and price changes rejected", db.Name(), id, expires.Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), time.Since(expires))
}
