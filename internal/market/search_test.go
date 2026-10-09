package market_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestSearchChineseSynonymsAndExplanations(t *testing.T) {
	base, request, _ := setupDatabase(t, time.Hour)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	for _, content := range [][2]string{{"自行车", "校园通勤"}, {"闲置物品", "单车"}, {"脚踏车", "脚踏车"}} {
		body := productBody("bicycles", map[string]any{"brand": "校园", "wheelSize": 26})
		body["title"], body["description"] = content[0], content[1]
		authRequest(t, base, "POST", "/products", token, bodyJSON(t, body), 201)
	}
	for _, q := range []string{"自行车", "单车", "脚踏车"} {
		result := awaitSearch(t, base, q, 3)
		if len(result["results"].([]any)) != 3 {
			t.Fatalf("results: %v", result)
		}
		for _, raw := range result["results"].([]any) {
			hit := raw.(map[string]any)
			expected := 1
			if hit["title"] == "脚踏车" {
				expected = 2
			}
			if len(hit["matchedFields"].([]any)) != expected || len(hit["highlights"].([]any)) != expected {
				t.Fatalf("explanation: %v", hit)
			}
			for _, rawHighlight := range hit["highlights"].([]any) {
				found := false
				for _, text := range rawHighlight.(map[string]any)["texts"].([]any) {
					if text.(map[string]any)["type"] == "hit" {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing hit fragment: %v", hit)
				}
			}
		}
	}
	for _, query := range []string{"", "?q=", "?q=%20", "?q=a&q=b", "?q=" + strings.Repeat("a", 301)} {
		request("GET", "/search"+query, "", 400)
	}
	empty := request("GET", "/search?q=量子火星", "", 200)
	if empty["total"] != float64(0) || len(empty["results"].([]any)) != 0 {
		t.Fatalf("empty: %v", empty)
	}
}

func awaitSearch(t *testing.T, base, query string, total int) map[string]any {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	var last map[string]any
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/search?q=" + url.QueryEscape(query))
		if err != nil {
			t.Fatal(err)
		}
		status := resp.StatusCode
		err = json.NewDecoder(resp.Body).Decode(&last)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("search decode (status %d): %v", status, err)
		}
		if status == 404 {
			t.Fatalf("search route missing: %v", last)
		}
		if status == 200 && last["total"] == float64(total) {
			return last
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("Search index readiness timeout query=%q expected=%d last=%v", query, total, last)
	return nil
}

func TestSearchRefillsAfterCurrentEligibilityFiltering(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	ctx := context.Background()
	if _, err := db.Collection("listings").Indexes().DropOne(ctx, "expiresAt_1"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := 0; i < 70; i++ {
		id := fmt.Sprintf("%024x", i+1)
		title, description := "自行车", "校园通勤"
		expiry := now.Add(time.Hour)
		if _, err := db.Collection("products").InsertOne(ctx, bson.M{"_id": id, "title": title, "description": description, "sold": false, "expiresAt": expiry}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Collection("listings").InsertOne(ctx, bson.M{"_id": id, "expiresAt": expiry}); err != nil {
			t.Fatal(err)
		}
	}
	indexed := awaitSearch(t, base, "单车", 70)
	// Invalidate the first 20 ranked candidates plus 25 others only after all
	// 70 are indexed, so filtering only the original top 20 cannot pass.
	ids := []string{}
	invalid := map[string]bool{}
	for _, raw := range indexed["results"].([]any) {
		id := raw.(map[string]any)["id"].(string)
		ids = append(ids, id)
		invalid[id] = true
	}
	for i := 0; len(ids) < 45; i++ {
		id := fmt.Sprintf("%024x", i+1)
		if !invalid[id] {
			ids = append(ids, id)
			invalid[id] = true
		}
	}
	for i, id := range ids {
		var err error
		switch i % 4 {
		case 0:
			_, err = db.Collection("products").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"expiresAt": now.Add(-time.Hour)}})
		case 1:
			_, err = db.Collection("products").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"sold": true}})
		case 2:
			_, err = db.Collection("listings").DeleteOne(ctx, bson.M{"_id": id})
		case 3:
			_, err = db.Collection("listings").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"expiresAt": now.Add(-time.Hour)}})
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	start := time.Now()
	result := request("GET", "/search?q=单车", "", 200)
	t.Logf("70 indexed candidates; 45 currently invalid; search response elapsed=%s", time.Since(start))
	if result["total"] != float64(25) {
		t.Fatalf("current eligibility total: %v", result)
	}
	if len(result["results"].([]any)) != 20 {
		t.Fatalf("not refilled: %v", result)
	}
	for _, raw := range result["results"].([]any) {
		hit := raw.(map[string]any)
		if invalid[hit["id"].(string)] {
			t.Fatalf("invalid candidate: %v", hit)
		}
	}
}

func TestSearchSwaggerContract(t *testing.T) {
	_, request := setup(t, time.Hour)
	spec := request("GET", "/openapi.json", "", 200)
	operation := spec["paths"].(map[string]any)["/search"].(map[string]any)["get"].(map[string]any)
	parameter := operation["parameters"].([]any)[0].(map[string]any)
	if parameter["name"] != "q" || parameter["required"] != true {
		t.Fatal("search query contract")
	}
	for _, status := range []string{"200", "400", "503"} {
		if operation["responses"].(map[string]any)[status] == nil {
			t.Fatal("missing status " + status)
		}
	}
}
