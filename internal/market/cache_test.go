package market_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nanfxqs/campus-market/internal/market"
	"go.mongodb.org/mongo-driver/bson"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func TestCachedDetailKeepsCurrentContentAndDynamicFields(t *testing.T) {
	_, _, db := setupDatabase(t, time.Hour)
	if os.Getenv("REDIS_ADDR") == "" {
		t.Skip("REDIS_ADDR required for real Redis HTTP tests")
	}
	server := httptest.NewServer(market.NewWithCache(db, time.Hour, market.RedisConfig{Address: os.Getenv("REDIS_ADDR"), TTL: time.Minute, Timeout: 100 * time.Millisecond}))
	defer server.Close()
	token := authRequest(t, server.URL, "POST", "/auth/login", "", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	body := productBody("textbooks", map[string]any{"author": "A"})
	p := authRequest(t, server.URL, "POST", "/products", token, bodyJSON(t, body), 201)
	path := "/products/" + p["id"].(string)
	authRequest(t, server.URL, "GET", path, "", "", 200)
	authRequest(t, server.URL, "GET", path, "", "", 200)
	delete(body, "priceCents")
	body["title"] = "Edited current title"
	authRequest(t, server.URL, "PUT", path, token, bodyJSON(t, body), 200)
	authRequest(t, server.URL, "PATCH", "/users/me", token, `{"nickname":"Current seller"}`, 200)
	got := authRequest(t, server.URL, "GET", path, "", "", 200)
	if got["product"].(map[string]any)["title"] != "Edited current title" || got["seller"].(map[string]any)["nickname"] != "Current seller" {
		t.Fatalf("stale detail: %v", got)
	}
}

func TestCacheConcurrentEditsExpiryResetAndUnavailableRedis(t *testing.T) {
	_, _, db := setupDatabase(t, time.Hour)
	config := market.RedisConfig{Address: os.Getenv("REDIS_ADDR"), TTL: 100 * time.Millisecond, Timeout: 50 * time.Millisecond}
	if config.Address == "" {
		t.Skip("REDIS_ADDR required for real Redis HTTP tests")
	}
	server := httptest.NewServer(market.NewWithCache(db, time.Hour, config))
	defer server.Close()
	token := authRequest(t, server.URL, "POST", "/auth/login", "", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	body := productBody("textbooks", map[string]any{"author": "A"})
	p := authRequest(t, server.URL, "POST", "/products", token, bodyJSON(t, body), 201)
	path := "/products/" + p["id"].(string)
	authRequest(t, server.URL, "GET", path, "", "", 200)
	delete(body, "priceCents")
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for j := 0; j < 20; j++ {
				response, err := http.Get(server.URL + path)
				if err != nil {
					t.Error(err)
					return
				}
				var result map[string]any
				err = json.NewDecoder(response.Body).Decode(&result)
				response.Body.Close()
				if err != nil || response.StatusCode != 200 {
					t.Errorf("concurrent read: %v status %d", err, response.StatusCode)
					return
				}
			}
		}()
	}
	for i := 0; i < 10; i++ {
		body["title"] = fmt.Sprintf("Committed title %d", i)
		authRequest(t, server.URL, "PUT", path, token, bodyJSON(t, body), 200)
		got := authRequest(t, server.URL, "GET", path, "", "", 200)
		if got["product"].(map[string]any)["title"] != body["title"] {
			t.Fatal("new read returned old version")
		}
	}
	readers.Wait()
	time.Sleep(150 * time.Millisecond)
	got := authRequest(t, server.URL, "GET", path, "", "", 200)
	if got["product"].(map[string]any)["title"] != "Committed title 9" {
		t.Fatal("expired entry returned stale content")
	}
	if err := market.ClearDetailCache(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	authRequest(t, server.URL, "GET", path, "", "", 200)
	// External expiry fixture is a pre-agreed test seam: qualification becomes false
	// immediately even while stable content remains in a warm cache.
	_, err := db.Collection("listings").UpdateOne(context.Background(), bson.M{"_id": p["id"]}, bson.M{"$set": bson.M{"expiresAt": time.Now().Add(-time.Hour)}})
	if err != nil {
		t.Fatal(err)
	}
	got = authRequest(t, server.URL, "GET", path, "", "", 200)
	if got["available"] != false {
		t.Fatal("cached eligibility")
	}
	broken := httptest.NewServer(market.NewWithCache(db, time.Hour, market.RedisConfig{Address: "127.0.0.1:1", Timeout: 50 * time.Millisecond}))
	defer broken.Close()
	start := time.Now()
	got = authRequest(t, broken.URL, "GET", path, "", "", 200)
	if got["product"].(map[string]any)["title"] != "Committed title 9" || time.Since(start) > time.Second {
		t.Fatal("unbounded or stale Redis fallback")
	}
	t.Log("8 readers x 20 reads overlap 10 committed HTTP edits; new reads, TTL, FLUSHDB rebuild, expiry fixture and unreachable Redis passed")
}
func TestDetailMetricsObserveColdAndWarmReads(t *testing.T) {
	_, _, db := setupDatabase(t, time.Hour)
	config := market.RedisConfig{Address: os.Getenv("REDIS_ADDR")}
	if config.Address == "" {
		t.Skip("REDIS_ADDR required for real Redis HTTP tests")
	}
	server := httptest.NewServer(market.NewWithCache(db, time.Hour, config))
	defer server.Close()
	token := authRequest(t, server.URL, "POST", "/auth/login", "", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	p := authRequest(t, server.URL, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)
	path := server.URL + "/products/" + p["id"].(string)
	for _, want := range []struct{ state, queries string }{{"miss", "5"}, {"hit", "4"}} {
		resp, err := http.Get(path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.Header.Get("X-Detail-Cache") != want.state || resp.Header.Get("X-Mongo-Queries") != want.queries || resp.Header.Get("X-Mongo-BSON-Bytes") == "" {
			t.Fatalf("metrics: %v", resp.Header)
		}
		t.Logf("state=%s queries=%s returned_bson_bytes=%s", want.state, want.queries, resp.Header.Get("X-Mongo-BSON-Bytes"))
	}
}
