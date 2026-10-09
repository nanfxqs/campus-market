package market_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestConditionCounts(t *testing.T) {
	_, request, db := setupDatabase(t, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.Collection("listings").Indexes().DropOne(ctx, "expiresAt_1"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	conditions := []string{"全新", "几乎全新", "轻度使用", "明显使用"}
	id := 0
	for grade, condition := range conditions {
		for n := 0; n <= grade; n++ {
			id++
			key := fmt.Sprintf("%024x", id)
			if _, err := db.Collection("products").InsertOne(ctx, bson.M{"_id": key, "categoryId": "textbooks", "condition": condition, "sold": false, "expiresAt": now.Add(time.Hour)}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Collection("listings").InsertOne(ctx, bson.M{"_id": key, "expiresAt": now.Add(time.Hour)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, state := range []string{"other", "sold", "delisted", "expiredArchive", "expiredListing"} {
		id++
		key := fmt.Sprintf("%024x", id)
		category, expiry := "textbooks", now.Add(time.Hour)
		if state == "other" {
			category = "bicycles"
		}
		if state == "expiredArchive" {
			expiry = now
		}
		if _, err := db.Collection("products").InsertOne(ctx, bson.M{"_id": key, "categoryId": category, "condition": "全新", "sold": state == "sold", "expiresAt": expiry}); err != nil {
			t.Fatal(err)
		}
		if state != "delisted" {
			listingExpiry := now.Add(time.Hour)
			if state == "expiredListing" {
				listingExpiry = now
			}
			if _, err := db.Collection("listings").InsertOne(ctx, bson.M{"_id": key, "expiresAt": listingExpiry}); err != nil {
				t.Fatal(err)
			}
		}
	}
	assertCounts := func(category string, counts []int) {
		t.Helper()
		result := request("GET", "/categories/"+category+"/condition-counts", "", 200)
		want := []any{}
		for i, condition := range conditions {
			want = append(want, map[string]any{"condition": condition, "count": float64(counts[i])})
		}
		if result["categoryId"] != category || !reflect.DeepEqual(result["items"], want) {
			t.Fatalf("counts: %v, want %v", result, want)
		}
	}
	assertCounts("textbooks", []int{1, 2, 3, 4})
	assertCounts("bicycles", []int{1, 0, 0, 0})
	assertCounts("electronics", []int{0, 0, 0, 0})
	request("GET", "/categories/unknown/condition-counts", "", 404)
}

func TestConditionCountsOpenAPI(t *testing.T) {
	_, request := setup(t, time.Hour)
	spec := request("GET", "/openapi.json", "", 200)
	operation := spec["paths"].(map[string]any)["/categories/{id}/condition-counts"].(map[string]any)["get"].(map[string]any)
	if operation["operationId"] != "categoryConditionCounts" {
		t.Fatal("missing statistics operation")
	}
	for _, status := range []string{"200", "400", "404", "503"} {
		if operation["responses"].(map[string]any)[status] == nil {
			t.Fatal("missing response " + status)
		}
	}
	items := spec["components"].(map[string]any)["schemas"].(map[string]any)["ConditionCounts"].(map[string]any)["properties"].(map[string]any)["items"].(map[string]any)
	if items["minItems"] != float64(4) || items["maxItems"] != float64(4) {
		t.Fatal("four grades undocumented")
	}
	request("GET", "/categories/"+strings.Repeat("x", 65)+"/condition-counts", "", 400)
}
