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

func TestBrowseFiltersUnavailableAndOtherCategories(t *testing.T) {
	_, request, db := setupDatabase(t, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Millisecond)
	if _, err := db.Collection("listings").Indexes().DropOne(ctx, "expiresAt_1"); err != nil {
		t.Fatal(err)
	}
	// Suspend physical cleanup in this isolated fixture to prove pending TTL is excluded.
	// Fixtures include sale states whose write APIs belong to later tickets.
	for i, state := range []string{"valid", "other", "sold", "delisted", "expiredArchive", "expiredListing"} {
		id := fmt.Sprintf("%024x", i+1)
		category := "textbooks"
		if state == "other" {
			category = "bicycles"
		}
		expiry := now.Add(time.Hour)
		if state == "expiredArchive" {
			expiry = now
		}
		p := bson.M{"_id": id, "categoryId": category, "publishedAt": now, "expiresAt": expiry, "sold": state == "sold", "title": state}
		if _, err := db.Collection("products").InsertOne(ctx, p); err != nil {
			t.Fatal(err)
		}
		if state != "delisted" {
			listingExpiry := now.Add(time.Hour)
			if state == "expiredListing" {
				listingExpiry = now
			}
			if _, err := db.Collection("listings").InsertOne(ctx, bson.M{"_id": id, "expiresAt": listingExpiry}); err != nil {
				t.Fatal(err)
			}
		}
	}
	page := request("GET", "/products?categoryId=textbooks", "", 200)
	items := page["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["title"] != "valid" || page["nextCursor"] != "" {
		t.Fatalf("unexpected page: %v", page)
	}
	empty := request("GET", "/products?categoryId=electronics", "", 200)
	if len(empty["items"].([]any)) != 0 || empty["nextCursor"] != "" {
		t.Fatalf("unexpected empty page: %v", empty)
	}
}

func TestBrowseStablePaginationAndLimits(t *testing.T) {
	_, request, db := setupDatabase(t, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Millisecond)
	products, listings := []any{}, []any{}
	for i := 1; i <= 105; i++ {
		id := fmt.Sprintf("%024x", i)
		published := now
		if i == 1 {
			published = now.Add(time.Second)
		}
		products = append(products, bson.M{"_id": id, "categoryId": "textbooks", "sold": false, "publishedAt": published, "expiresAt": now.Add(time.Hour)})
		listings = append(listings, bson.M{"_id": id, "expiresAt": now.Add(time.Hour)})
	}
	if _, err := db.Collection("products").InsertMany(ctx, products); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("listings").InsertMany(ctx, listings); err != nil {
		t.Fatal(err)
	}
	first := request("GET", "/products?categoryId=textbooks", "", 200)
	if len(first["items"].([]any)) != 20 || first["nextCursor"] == "" {
		t.Fatalf("default page: %v", first)
	}
	max := request("GET", "/products?categoryId=textbooks&limit=100", "", 200)
	if len(max["items"].([]any)) != 100 {
		t.Fatal("maximum page size")
	}
	seen := []string{}
	cursor := ""
	for page := 0; page < 16; page++ {
		path := "/products?categoryId=textbooks&limit=7"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		result := request("GET", path, "", 200)
		for _, raw := range result["items"].([]any) {
			seen = append(seen, raw.(map[string]any)["id"].(string))
		}
		cursor = result["nextCursor"].(string)
		if cursor == "" {
			break
		}
	}
	if len(seen) != 105 || cursor != "" {
		t.Fatalf("missing or duplicate rows: %v", seen)
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
	for _, query := range []string{"", "?categoryId=", "?categoryId=unknown", "?categoryId=textbooks&limit=0", "?categoryId=textbooks&limit=101", "?categoryId=textbooks&limit=abc", "?categoryId=textbooks&limit=", "?categoryId=textbooks&cursor=", "?categoryId=textbooks&cursor=broken"} {
		status := 400
		if query == "?categoryId=unknown" {
			status = 404
		}
		request("GET", "/products"+query, "", status)
	}
	for _, query := range []string{"&categoryId=bicycles", "&limit=1&limit=2", "&cursor=x&cursor=y", "&categoryId=" + strings.Repeat("x", 65)} {
		request("GET", "/products?categoryId=textbooks"+query, "", 400)
	}
	for _, raw := range []string{`{}`, `{"categoryId":"textbooks","publishedAt":"2026-01-01T00:00:00Z","id":"bad"}`, `{"categoryId":"textbooks","publishedAt":"2026-01-01T00:00:00.000001Z","id":"000000000000000000000001"}`, `{"categoryId":"textbooks","publishedAt":"2026-01-01T00:00:00Z","id":"000000000000000000000001","extra":true}`, `{"categoryId":"textbooks","publishedAt":"2026-01-01T00:00:00Z","id":"000000000000000000000001"} {}`} {
		request("GET", "/products?categoryId=textbooks&cursor="+base64.RawURLEncoding.EncodeToString([]byte(raw)), "", 400)
	}
	request("GET", "/products?categoryId=bicycles&cursor="+first["nextCursor"].(string), "", 400)
}

func TestBrowseSwaggerContract(t *testing.T) {
	_, request := setup(t, time.Hour)
	spec := request("GET", "/openapi.json", "", 200)
	operation := spec["paths"].(map[string]any)["/products"].(map[string]any)["get"].(map[string]any)
	params := operation["parameters"].([]any)
	if len(params) != 3 || params[0].(map[string]any)["required"] != true {
		t.Fatal("category contract missing")
	}
	limit := params[1].(map[string]any)["schema"].(map[string]any)
	if limit["default"] != float64(20) || limit["maximum"] != float64(100) {
		t.Fatal("pagination limits missing")
	}
	for _, status := range []string{"200", "400", "404", "503"} {
		if operation["responses"].(map[string]any)[status] == nil {
			t.Fatal("missing response " + status)
		}
	}
}
