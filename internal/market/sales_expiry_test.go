package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
			request := func(path, body, token string, want int) map[string]any {
				t.Helper()
				req, _ := http.NewRequest("POST", server.URL+path, bytes.NewBufferString(body))
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
