package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/event"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Time is controlled only at the clock boundary. Requests still traverse the
// real HTTP API and MongoDB transactions; no exported test API is introduced.
func TestSaleExactExpirationHTTP(t *testing.T) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Fatal("MONGO_URI required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database(fmt.Sprintf("campus_sale_boundary_%d", time.Now().UnixNano()))
	t.Cleanup(func() { db.Drop(context.Background()); client.Disconnect(context.Background()) })
	if err := Seed(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("listings").Indexes().DropOne(ctx, "expiresAt_1"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().UTC().Truncate(time.Millisecond)
	for _, offset := range []time.Duration{-time.Millisecond, 0, time.Millisecond} {
		t.Run(offset.String(), func(t *testing.T) {
			now := deadline.Add(offset)
			server := httptest.NewServer(newAPI(db, time.Hour, func() time.Time { return now }))
			defer server.Close()
			request := saleBoundaryRequest(t, server.URL)
			token := request("/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, "", 200)["accessToken"].(string)
			id := primitive.NewObjectID().Hex()
			if _, err := db.Collection("products").InsertOne(ctx, Product{ID: id, SellerID: "seller", PublishedAt: deadline.Add(-1440 * time.Hour), ExpiresAt: deadline, Sold: false}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Collection("listings").InsertOne(ctx, bson.M{"_id": id, "expiresAt": deadline}); err != nil {
				t.Fatal(err)
			}
			want := 409
			if offset < 0 {
				want = 200
			}
			result := request("/products/"+id+"/sale", fmt.Sprintf(`{"buyerId":"buyer","priceCents":101,"idempotencyKey":%q}`, id), token, want)
			if offset >= 0 && result["code"] != "product_expired" {
				t.Fatal(result)
			}
			if result["confirmedAt"] != now.Format(time.RFC3339Nano) {
				t.Fatalf("confirmation clock mismatch: %v", result)
			}
			t.Logf("expiry=%s confirmation=%s code=%s", deadline.Format(time.RFC3339Nano), result["confirmedAt"], result["code"])
		})
	}
}

func TestSaleTransactionRetryAcrossExpirationHTTP(t *testing.T) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Fatal("MONGO_URI required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	control, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("campus_sale_retry_expiry_%d", time.Now().UnixNano())
	fixture := control.Database(name)
	t.Cleanup(func() { fixture.Drop(context.Background()); control.Disconnect(context.Background()) })
	if err := Seed(ctx, fixture); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)
	var clock atomic.Int64
	clock.Store(deadline.Add(-time.Millisecond).UnixMilli())
	id := primitive.NewObjectID().Hex()
	if _, err := fixture.Collection("products").InsertOne(ctx, Product{ID: id, SellerID: "seller", PublishedAt: deadline.Add(-1440 * time.Hour), ExpiresAt: deadline, Sold: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.Collection("listings").InsertOne(ctx, bson.M{"_id": id, "expiresAt": deadline}); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	var competitorErr error
	var competed atomic.Bool
	monitor := &event.CommandMonitor{Started: func(_ context.Context, e *event.CommandStartedEvent) {
		if e.CommandName != "update" || e.Command.Lookup("update").StringValue() != "products" {
			return
		}
		once.Do(func() {
			// A separate MongoDB client changes the archive after the sale transaction's
			// snapshot read, forcing the first conditional update to retry.
			_, competitorErr = fixture.Collection("products").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"title": "并发修改"}})
			clock.Store(deadline.UnixMilli())
			competed.Store(true)
		})
	}}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetMonitor(monitor))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) })
	server := httptest.NewServer(newAPI(client.Database(name), time.Hour, func() time.Time { return time.UnixMilli(clock.Load()) }))
	defer server.Close()
	request := saleBoundaryRequest(t, server.URL)
	token := request("/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, "", 200)["accessToken"].(string)
	body := `{"buyerId":"buyer","priceCents":101,"idempotencyKey":"retry-expiry"}`
	result := request("/products/"+id+"/sale", body, token, 409)
	if !competed.Load() || competitorErr != nil {
		t.Fatalf("competing write: observed=%v err=%v", competed.Load(), competitorErr)
	}
	if result["code"] != "product_expired" || result["confirmedAt"] != deadline.Format(time.RFC3339Nano) {
		t.Fatalf("retry did not recheck time: %v", result)
	}
	if replay := request("/products/"+id+"/sale", body, token, 409); !reflect.DeepEqual(result, replay) {
		t.Fatal("expiry failure not replayed")
	}
	var product Product
	if err := fixture.Collection("products").FindOne(ctx, bson.M{"_id": id}).Decode(&product); err != nil {
		t.Fatal(err)
	}
	if product.Sold || product.Sale != nil {
		t.Fatalf("partial sale: %+v", product)
	}
	n, err := fixture.Collection("listings").CountDocuments(ctx, bson.M{"_id": id})
	if err != nil || n != 1 {
		t.Fatalf("eligibility=%d err=%v", n, err)
	}
	n, err = fixture.Collection("transactions").CountDocuments(ctx, bson.M{"productId": id})
	if err != nil || n != 1 {
		t.Fatalf("attempts=%d err=%v", n, err)
	}
	n, err = fixture.Collection("transactions").CountDocuments(ctx, bson.M{"success": true})
	if err != nil || n != 0 {
		t.Fatalf("success records=%d err=%v", n, err)
	}
	var seller Profile
	if err := fixture.Collection("users").FindOne(ctx, bson.M{"_id": "seller"}).Decode(&seller); err != nil || seller.CompletedSales != 0 {
		t.Fatalf("counter=%d err=%v", seller.CompletedSales, err)
	}
	t.Log("concurrent write forced transaction retry at expiry: no sale, eligibility retained, one replayable failure, no successful record or counter increment")
}

func saleBoundaryRequest(t *testing.T, base string) func(string, string, string, int) map[string]any {
	return func(path, body, token string, want int) map[string]any {
		t.Helper()
		req, _ := http.NewRequest("POST", base+path, bytes.NewBufferString(body))
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
			t.Fatalf("status=%d want=%d %v", resp.StatusCode, want, result)
		}
		return result
	}
}
