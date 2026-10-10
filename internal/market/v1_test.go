package market_test

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"testing"
	"time"
)

// This deliberately changes the disposable full-seed acceptance database.
func TestV1FullSeedHTTP(t *testing.T) {
	base := os.Getenv("V1_API_URL")
	if base == "" {
		t.Skip("run scripts/validate-v1.sh for full-seed deployment acceptance")
	}
	search := awaitSearch(t, base, "单车", 6667)
	if len(search["results"].([]any)) != 20 {
		t.Fatal(search)
	}
	for _, raw := range search["results"].([]any) {
		row := raw.(map[string]any)
		if len(row["matchedFields"].([]any)) == 0 || len(row["highlights"].([]any)) == 0 {
			t.Fatal(row)
		}
	}
	t.Log("full seed: synonym search total=6667, top 20 with matched fields")
	browse := authRequest(t, base, "GET", "/products?categoryId=bicycles", "", "", 200)
	if len(browse["items"].([]any)) != 20 || browse["nextCursor"] == "" {
		t.Fatal(browse)
	}
	stats := authRequest(t, base, "GET", "/categories/bicycles/condition-counts", "", "", 200)
	total := float64(0)
	for _, raw := range stats["items"].([]any) {
		total += raw.(map[string]any)["count"].(float64)
	}
	if total != 6667 || len(stats["items"].([]any)) != 4 {
		t.Fatal(stats)
	}
	seeded := authRequest(t, base, "GET", "/products/000000000000000000000001", "", "", 200)
	if seeded["available"] != true || len(seeded["priceHistory"].([]any)) == 0 || len(seeded["product"].(map[string]any)["images"].([]any)) == 0 {
		t.Fatal(seeded)
	}
	seller := authRequest(t, base, "POST", "/auth/login", "", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	buyer := authRequest(t, base, "POST", "/auth/login", "", `{"username":"buyer","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	p := authRequest(t, base, "POST", "/products", seller, bodyJSON(t, productBody("textbooks", map[string]any{"author": "V1验收"})), 201)
	path := "/products/" + p["id"].(string)
	authRequest(t, base, "PATCH", path+"/price", buyer, `{"priceCents":12000}`, 403)
	authRequest(t, base, "PATCH", path+"/price", seller, `{"priceCents":12000}`, 200)
	authRequest(t, base, "PATCH", "/users/me", seller, `{"nickname":"V1当前卖家"}`, 200)
	detail := authRequest(t, base, "GET", path, "", "", 200)
	if detail["seller"].(map[string]any)["nickname"] != "V1当前卖家" || len(detail["priceHistory"].([]any)) != 1 || detail["product"].(map[string]any)["expiresAt"] != p["expiresAt"] {
		t.Fatal(detail)
	}
	home := authRequest(t, base, "GET", "/users/seller/home", "", "", 200)
	body := bodyJSON(t, map[string]any{"buyerId": "buyer", "priceCents": 10001, "idempotencyKey": "v1-" + p["id"].(string)})
	authRequest(t, base, "POST", path+"/sale", buyer, `{"buyerId":"seller","priceCents":10001,"idempotencyKey":"non-owner"}`, 403)
	sale := authRequest(t, base, "POST", path+"/sale", seller, body, 200)
	replay := authRequest(t, base, "POST", path+"/sale", seller, body, 200)
	if bodyJSON(t, sale) != bodyJSON(t, replay) {
		t.Fatal("replay changed result")
	}
	after := authRequest(t, base, "GET", "/users/seller/home", "", "", 200)
	if after["completedSales"].(float64) != home["completedSales"].(float64)+1 || after["currentOnSaleCount"].(float64) != home["currentOnSaleCount"].(float64)-1 {
		t.Fatal(after)
	}
	if authRequest(t, base, "GET", path, "", "", 200)["available"] != false {
		t.Fatal("sold product remains available")
	}
	authRequest(t, base, "GET", "/openapi.json", "", "", 200)
	t.Log("full seed: publish, browse, detail/current profile, price history, seller home, sale/replay and ownership PASS")
	t.Run("full-seed expiry", func(t *testing.T) {
		// External fixture preparation at the agreed MongoDB boundary, never a public test API.
		if os.Getenv("V1_DB_NAME") != "campus_v1" {
			t.Fatal("V1_DB_NAME=campus_v1 required for disposable expiry fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
		if err != nil {
			t.Fatal(err)
		}
		defer client.Disconnect(context.Background())
		db := client.Database("campus_v1")
		// Known full-seed bicycle belonging to seller; keep its original images and history.
		id := "000000000000000000002711"
		path := "/products/" + id
		before := authRequest(t, base, "GET", path, "", "", 200)
		if before["available"] != true || before["product"].(map[string]any)["sellerId"] != "seller" {
			t.Fatal(before)
		}
		oldHome := authRequest(t, base, "GET", "/users/seller/home", "", "", 200)
		expires := time.Now().UTC().Add(-time.Second)
		for _, collection := range []string{"products", "listings"} {
			fields := bson.M{"expiresAt": expires}
			if collection == "products" {
				fields["publishedAt"] = expires.Add(-1440 * time.Hour)
			}
			result, err := db.Collection(collection).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": fields})
			if err != nil || result.MatchedCount != 1 {
				t.Fatalf("expiry fixture %s: result=%v err=%v", collection, result, err)
			}
		}
		expired := authRequest(t, base, "GET", path, "", "", 200)
		if expired["available"] != false || bodyJSON(t, expired["priceHistory"]) != bodyJSON(t, before["priceHistory"]) || bodyJSON(t, expired["product"].(map[string]any)["images"]) != bodyJSON(t, before["product"].(map[string]any)["images"]) {
			t.Fatal(expired)
		}
		authRequest(t, base, "PATCH", path+"/price", seller, `{"priceCents":10001}`, 409)
		authRequest(t, base, "POST", path+"/sale", seller, `{"buyerId":"buyer","priceCents":10001,"idempotencyKey":"v1-expired"}`, 409)
		awaitSearch(t, base, "单车", 6666)
		stats := authRequest(t, base, "GET", "/categories/bicycles/condition-counts", "", "", 200)
		total := float64(0)
		for _, raw := range stats["items"].([]any) {
			total += raw.(map[string]any)["count"].(float64)
		}
		if total != 6666 {
			t.Fatal(stats)
		}
		cursor, count := "", 0
		for page := 0; page < 100; page++ {
			url := "/products?categoryId=bicycles&limit=100"
			if cursor != "" {
				url += "&cursor=" + cursor
			}
			result := authRequest(t, base, "GET", url, "", "", 200)
			for _, raw := range result["items"].([]any) {
				if raw.(map[string]any)["id"] == id {
					t.Fatal("expired bicycle still browsable")
				}
				count++
			}
			cursor = result["nextCursor"].(string)
			if cursor == "" {
				break
			}
		}
		if count != 6666 || cursor != "" {
			t.Fatalf("expired browse count=%d cursor=%q", count, cursor)
		}
		home := authRequest(t, base, "GET", "/users/seller/home", "", "", 200)
		if home["completedSales"] != oldHome["completedSales"] || home["currentOnSaleCount"].(float64) != oldHome["currentOnSaleCount"].(float64)-1 {
			t.Fatal(home)
		}
		t.Log("full seed: expired bicycle excluded from all browse pages, Search total and statistics; sale/price rejected; archive/images/history retained")
	})

}
