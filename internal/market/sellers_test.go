package market_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestSellerHomeCurrentProfileAndIsolation(t *testing.T) {
	base, request := setup(t, time.Hour)
	seller := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	buyer := request("POST", "/auth/login", `{"username":"buyer","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	id := authRequest(t, base, "POST", "/products", seller, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)["id"]
	authRequest(t, base, "POST", "/products", buyer, bodyJSON(t, productBody("textbooks", map[string]any{"author": "B"})), 201)
	authRequest(t, base, "PATCH", "/users/me", seller, `{"nickname":"当前卖家","avatar":"https://example.com/new.png"}`, 200)
	home := request("GET", "/users/seller/home", "", 200)
	profile := home["profile"].(map[string]any)
	if profile["id"] != "seller" || profile["nickname"] != "当前卖家" || profile["avatar"] != "https://example.com/new.png" || profile["passwordHash"] != nil {
		t.Fatalf("current public profile: %v", home)
	}
	items := home["items"].([]any)
	if home["creditScore"] != float64(100) || home["completedSales"] != float64(0) || home["currentOnSaleCount"] != float64(1) || len(items) != 1 || items[0].(map[string]any)["id"] != id || home["nextCursor"] != "" {
		t.Fatalf("seller isolation: %v", home)
	}
	request("GET", "/users/unknown/home", "", 404)
}

func assertSellerHomeCounts(t *testing.T, base, seller string, sales, onSale int) {
	t.Helper()
	home := authRequest(t, base, "GET", "/users/"+seller+"/home", "", "", 200)
	if home["completedSales"] != float64(sales) || home["currentOnSaleCount"] != float64(onSale) || home["creditScore"] != float64(100) || len(home["items"].([]any)) != onSale {
		t.Fatalf("seller home after sale operation: %v", home)
	}
}

func TestSellerHomePaginationAndInvalidInput(t *testing.T) {
	_, request, db := setupDatabase(t, time.Hour)
	now := time.Now().UTC().Truncate(time.Millisecond)
	products, listings := []any{}, []any{}
	for i := 1; i <= 105; i++ {
		id := fmt.Sprintf("%024x", i)
		published := now
		if i == 1 {
			published = now.Add(time.Second)
		}
		category := "textbooks"
		if i%2 == 0 {
			category = "electronics"
		}
		products = append(products, bson.M{"_id": id, "sellerId": "seller", "categoryId": category, "sold": false, "publishedAt": published, "expiresAt": now.Add(time.Hour)})
		listings = append(listings, bson.M{"_id": id, "expiresAt": now.Add(time.Hour)})
	}
	if _, err := db.Collection("products").InsertMany(context.Background(), products); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("listings").InsertMany(context.Background(), listings); err != nil {
		t.Fatal(err)
	}
	first := request("GET", "/users/seller/home", "", 200)
	if len(first["items"].([]any)) != 20 || first["nextCursor"] == "" || first["currentOnSaleCount"] != float64(105) {
		t.Fatalf("default page: %v", first)
	}
	max := request("GET", "/users/seller/home?limit=100", "", 200)
	if len(max["items"].([]any)) != 100 {
		t.Fatal("maximum limit")
	}
	cursor := ""
	seen := []string{}
	for page := 0; page < 16; page++ {
		path := "/users/seller/home?limit=7"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		result := request("GET", path, "", 200)
		if result["currentOnSaleCount"] != float64(105) {
			t.Fatal("count was limited by cursor")
		}
		for _, raw := range result["items"].([]any) {
			seen = append(seen, raw.(map[string]any)["id"].(string))
		}
		cursor = result["nextCursor"].(string)
		if cursor == "" {
			break
		}
	}
	if len(seen) != 105 || cursor != "" {
		t.Fatalf("missing or duplicate items: %v", seen)
	}
	for i, id := range seen {
		want := 1
		if i > 0 {
			want = 106 - i
		}
		if id != fmt.Sprintf("%024x", want) {
			t.Fatalf("order at %d: %s", i, id)
		}
	}
	empty := request("GET", "/users/buyer/home", "", 200)
	if len(empty["items"].([]any)) != 0 || empty["currentOnSaleCount"] != float64(0) || empty["nextCursor"] != "" {
		t.Fatalf("empty home: %v", empty)
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=abc", "limit=", "limit=1&limit=2", "cursor=", "cursor=broken", "cursor=x&cursor=y", "cursor=" + strings.Repeat("x", 513)} {
		request("GET", "/users/seller/home?"+query, "", 400)
	}
	request("GET", "/users/"+strings.Repeat("x", 65)+"/home", "", 400)
	request("GET", "/users/buyer/home?cursor="+first["nextCursor"].(string), "", 400)
	request("GET", "/products?categoryId=textbooks&cursor="+first["nextCursor"].(string), "", 400)
	browse := request("GET", "/products?categoryId=textbooks", "", 200)
	request("GET", "/users/seller/home?cursor="+browse["nextCursor"].(string), "", 400)
	for _, raw := range []string{`{}`, `{"sellerId":"seller","publishedAt":"2026-01-01T00:00:00Z","id":"bad"}`, `{"sellerId":"seller","publishedAt":"2026-01-01T00:00:00.000001Z","id":"000000000000000000000001"}`, `{"sellerId":"seller","publishedAt":"2026-01-01T00:00:00Z","id":"000000000000000000000001","extra":true}`, `{"sellerId":"seller","publishedAt":"2026-01-01T00:00:00Z","id":"000000000000000000000001"} {}`} {
		request("GET", "/users/seller/home?cursor="+base64.RawURLEncoding.EncodeToString([]byte(raw)), "", 400)
	}
}

func TestSellerHomeExcludesUnavailableBeforeTTLCleanup(t *testing.T) {
	_, request, db := setupDatabase(t, time.Hour)
	ctx := context.Background()
	if _, err := db.Collection("listings").Indexes().DropOne(ctx, "expiresAt_1"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	fixtures := []struct {
		name          string
		seller        string
		sold          bool
		archiveExpiry time.Time
		listingExpiry time.Time
	}{
		{name: "valid", seller: "seller", archiveExpiry: now.Add(time.Hour), listingExpiry: now.Add(time.Hour)},
		{name: "other", seller: "buyer", archiveExpiry: now.Add(time.Hour), listingExpiry: now.Add(time.Hour)},
		{name: "sold", seller: "seller", sold: true, archiveExpiry: now.Add(time.Hour), listingExpiry: now.Add(time.Hour)},
		{name: "delisted", seller: "seller", archiveExpiry: now.Add(time.Hour)},
		{name: "expiredArchive", seller: "seller", archiveExpiry: now, listingExpiry: now.Add(time.Hour)},
		{name: "expiredListing", seller: "seller", archiveExpiry: now.Add(time.Hour), listingExpiry: now},
	}
	for i, fixture := range fixtures {
		id := fmt.Sprintf("%024x", i+1)
		if _, err := db.Collection("products").InsertOne(ctx, bson.M{"_id": id, "sellerId": fixture.seller, "categoryId": "textbooks", "publishedAt": now, "expiresAt": fixture.archiveExpiry, "sold": fixture.sold, "title": fixture.name}); err != nil {
			t.Fatal(err)
		}
		if !fixture.listingExpiry.IsZero() {
			if _, err := db.Collection("listings").InsertOne(ctx, bson.M{"_id": id, "expiresAt": fixture.listingExpiry}); err != nil {
				t.Fatal(err)
			}
		}
	}
	home := request("GET", "/users/seller/home", "", 200)
	items := home["items"].([]any)
	if home["currentOnSaleCount"] != float64(1) || len(items) != 1 || items[0].(map[string]any)["title"] != "valid" || home["completedSales"] != float64(0) {
		t.Fatalf("TTL pending home: %v", home)
	}
	// Advance the remaining product's fixture deadline without deleting its listing.
	if _, err := db.Collection("products").UpdateOne(ctx, bson.M{"_id": "000000000000000000000001"}, bson.M{"$set": bson.M{"expiresAt": now}}); err != nil {
		t.Fatal(err)
	}
	after := request("GET", "/users/seller/home", "", 200)
	if after["currentOnSaleCount"] != float64(0) || len(after["items"].([]any)) != 0 || after["completedSales"] != float64(0) || after["creditScore"] != float64(100) {
		t.Fatalf("expiry changes only on-sale set: %v", after)
	}
}

func TestSellerHomeOpenAPI(t *testing.T) {
	_, request := setup(t, time.Hour)
	spec := request("GET", "/openapi.json", "", 200)
	path, ok := spec["paths"].(map[string]any)["/users/{id}/home"].(map[string]any)
	if !ok {
		t.Fatal("seller home missing from OpenAPI")
	}
	op := path["get"].(map[string]any)
	if security, ok := op["security"].([]any); ok && len(security) != 0 {
		t.Fatal("home should be public")
	}
	params := op["parameters"].([]any)
	if len(params) != 3 || params[0].(map[string]any)["required"] != true {
		t.Fatal("home path/pagination parameters missing")
	}
	limit := params[1].(map[string]any)["schema"].(map[string]any)
	if limit["default"] != float64(20) || limit["maximum"] != float64(100) {
		t.Fatal("home pagination limits missing")
	}
	for _, status := range []string{"200", "400", "404", "503"} {
		if op["responses"].(map[string]any)[status] == nil {
			t.Fatal("missing response " + status)
		}
	}
}
