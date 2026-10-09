package market_test

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSaleConfirmationAndReplay(t *testing.T) {
	base, request, _ := setupDatabase(t, time.Hour)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	p := authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)
	id := p["id"].(string)
	body := `{"buyerId":"buyer","priceCents":10001,"idempotencyKey":"first-sale"}`
	before := time.Now().Add(-time.Millisecond)
	result := authRequest(t, base, "POST", "/products/"+id+"/sale", token, body, 200)
	replay := authRequest(t, base, "POST", "/products/"+id+"/sale", token, body, 200)
	if bodyJSON(t, result) != bodyJSON(t, replay) || result["success"] != true || result["priceCents"] != float64(10001) {
		t.Fatalf("sale/replay: %v %v", result, replay)
	}
	confirmed, err := time.Parse(time.RFC3339Nano, result["confirmedAt"].(string))
	if err != nil || confirmed.Before(before) || confirmed.After(time.Now()) {
		t.Fatalf("server confirmation time: %v", result)
	}
	detail := request("GET", "/products/"+id, "", 200)
	if detail["available"] != false || detail["product"].(map[string]any)["sold"] != true || detail["seller"].(map[string]any)["completedSales"] != float64(1) {
		t.Fatalf("sale state: %v", detail)
	}
	authRequest(t, base, "POST", "/products/"+id+"/sale", token, `{"buyerId":"buyer","priceCents":10002,"idempotencyKey":"first-sale"}`, 409)
}

func TestSaleInvalidAttemptsAreNotRecorded(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	seller := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	buyer := request("POST", "/auth/login", `{"username":"buyer","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	id := authRequest(t, base, "POST", "/products", seller, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)["id"].(string)
	authRequest(t, base, "POST", "/products", seller, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A", "sale": "fake"})), 400)
	path := "/products/" + id + "/sale"
	authRequest(t, base, "POST", path, "", `{}`, 401)
	authRequest(t, base, "POST", path, buyer, `{"buyerId":"seller","priceCents":1,"idempotencyKey":"other"}`, 403)
	for _, body := range []string{`{}`, `{"buyerId":"buyer","priceCents":0,"idempotencyKey":"zero"}`, `{"buyerId":"buyer","priceCents":-1,"idempotencyKey":"negative"}`, `{"buyerId":"buyer","priceCents":1.5,"idempotencyKey":"fraction"}`, `{"buyerId":"seller","priceCents":1,"idempotencyKey":"self"}`, `{"buyerId":"unknown","priceCents":1,"idempotencyKey":"unknown"}`, `{"buyerId":"buyer","priceCents":1}`, `{"buyerId":"buyer","priceCents":1,"idempotencyKey":" "}`, `{"buyerId":"buyer","priceCents":1,"idempotencyKey":"key","confirmedAt":"2020-01-01"}`, `{"buyerId":"buyer","priceCents":1000000000001,"idempotencyKey":"max"}`} {
		authRequest(t, base, "POST", path, seller, body, 400)
	}
	saleCount(t, db, "transactions", bson.M{}, 0)
	if request("GET", "/products/"+id, "", 200)["available"] != true {
		t.Fatal("invalid attempt changed eligibility")
	}
}

func saleCount(t *testing.T, db *mongo.Database, collection string, filter bson.M, want int64) {
	t.Helper()
	n, err := db.Collection(collection).CountDocuments(context.Background(), filter)
	if err != nil || n != want {
		t.Fatalf("%s count=%d want=%d err=%v", collection, n, want, err)
	}
}

func TestSaleConcurrentExactlyOnce(t *testing.T) {
	for _, sameKey := range []bool{false, true} {
		t.Run(fmt.Sprintf("sameKey=%v", sameKey), func(t *testing.T) {
			base, request, db := setupDatabase(t, time.Hour)
			token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
			id := authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)["id"].(string)
			const n = 24
			results := make(chan saleHTTPResult, n)
			start := make(chan struct{})
			for i := 0; i < n; i++ {
				go func(i int) {
					<-start
					key := fmt.Sprintf("competition-%d", i)
					if sameKey {
						key = "shared"
					}
					body := fmt.Sprintf(`{"buyerId":"buyer","priceCents":4321,"idempotencyKey":%q}`, key)
					results <- sendSaleRequest(base, id, token, body)
				}(i)
			}
			close(start)
			successes := 0
			original := ""
			for i := 0; i < n; i++ {
				r := <-results
				if r.err != nil {
					t.Fatal(r.err)
				}
				switch r.status {
				case 200:
					successes++
					if original == "" {
						original = r.body
					}
					if sameKey && r.body != original {
						t.Fatalf("different replay: %s / %s", original, r.body)
					}
				case 409:
					if !strings.Contains(r.body, "product_sold") {
						t.Fatalf("business failure: %s", r.body)
					}
				default:
					t.Fatalf("concurrent status=%d %s", r.status, r.body)
				}
			}
			want := 1
			records := int64(n)
			if sameKey {
				want = n
				records = 1
			}
			if successes != want {
				t.Fatalf("success responses %d want %d", successes, want)
			}
			saleCount(t, db, "transactions", bson.M{"success": true}, 1)
			saleCount(t, db, "transactions", bson.M{}, records)
			saleCount(t, db, "listings", bson.M{"_id": id}, 0)
			detail := request("GET", "/products/"+id, "", 200)
			if detail["seller"].(map[string]any)["completedSales"] != float64(1) {
				t.Fatalf("counter: %v", detail)
			}
			failure := `{"buyerId":"buyer","priceCents":4321,"idempotencyKey":"fresh-failure"}`
			r1 := authRequest(t, base, "POST", "/products/"+id+"/sale", token, failure, 409)
			r2 := authRequest(t, base, "POST", "/products/"+id+"/sale", token, failure, 409)
			if bodyJSON(t, r1) != bodyJSON(t, r2) {
				t.Fatal("failure not replayed")
			}
			saleCount(t, db, "transactions", bson.M{}, records+1)
		})
	}
}

func TestSaleSellerKeyScopeAndProductConflict(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	seller := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	buyer := request("POST", "/auth/login", `{"username":"buyer","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	publish := func(token string) string {
		return authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)["id"].(string)
	}
	first, second, other := publish(seller), publish(seller), publish(buyer)
	body := `{"buyerId":"buyer","priceCents":1,"idempotencyKey":"scoped"}`
	authRequest(t, base, "POST", "/products/"+first+"/sale", seller, body, 200)
	conflict := authRequest(t, base, "POST", "/products/"+second+"/sale", seller, body, 409)
	if conflict["error"].(map[string]any)["code"] != "idempotency_conflict" {
		t.Fatal(conflict)
	}
	authRequest(t, base, "POST", "/products/"+other+"/sale", buyer, `{"buyerId":"seller","priceCents":1,"idempotencyKey":"scoped"}`, 200)
	saleCount(t, db, "transactions", bson.M{}, 2)
	if request("GET", "/products/"+second, "", 200)["available"] != true {
		t.Fatal("key conflict sold another product")
	}
}

func TestSaleUnavailableRecordsAndTTLDelay(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	ctx := context.Background()
	// Stop physical cleanup only in this isolated test DB, keeping expired eligibility observable.
	if _, err := db.Collection("listings").Indexes().DropOne(ctx, "expiresAt_1"); err != nil {
		t.Fatal(err)
	}
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	for _, kind := range []string{"before", "at", "after", "listing_expired", "delisted"} {
		t.Run(kind, func(t *testing.T) {
			id := authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)["id"].(string)
			deadline := time.Now().UTC().Truncate(time.Millisecond)
			switch kind {
			case "before":
				deadline = deadline.Add(time.Hour)
			case "after":
				deadline = deadline.Add(-time.Hour)
			}
			if kind != "delisted" && kind != "listing_expired" {
				if _, err := db.Collection("products").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"expiresAt": deadline}}); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "delisted" {
				if _, err := db.Collection("listings").DeleteOne(ctx, bson.M{"_id": id}); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := db.Collection("listings").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"expiresAt": deadline}}); err != nil {
					t.Fatal(err)
				}
			}
			body := fmt.Sprintf(`{"buyerId":"buyer","priceCents":123,"idempotencyKey":%q}`, kind)
			status := 409
			if kind == "before" {
				status = 200
			}
			first := authRequest(t, base, "POST", "/products/"+id+"/sale", token, body, status)
			if kind != "before" {
				code := "product_expired"
				if kind == "delisted" {
					code = "product_delisted"
				}
				if first["code"] != code {
					t.Fatal(first)
				}
				if kind != "delisted" {
					saleCount(t, db, "listings", bson.M{"_id": id}, 1)
				}
			}
			replay := authRequest(t, base, "POST", "/products/"+id+"/sale", token, body, status)
			if bodyJSON(t, first) != bodyJSON(t, replay) {
				t.Fatal("failure replay differs")
			}
			saleCount(t, db, "transactions", bson.M{"productId": id}, 1)
		})
	}
	saleCount(t, db, "transactions", bson.M{"success": true}, 1)
	profile := authRequest(t, base, "GET", "/users/me", token, "", 200)
	if profile["completedSales"] != float64(1) {
		t.Fatal(profile)
	}
}

func TestSaleRollbackOnPersistenceFailure(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	ctx := context.Background()
	// The last write is rejected after archive, eligibility and counter changes.
	if err := db.CreateCollection(ctx, "transactions", options.CreateCollection().SetValidator(bson.M{"success": false})); err != nil {
		t.Fatal(err)
	}
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	id := authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)["id"].(string)
	body := `{"buyerId":"buyer","priceCents":987,"idempotencyKey":"rollback"}`
	authRequest(t, base, "POST", "/products/"+id+"/sale", token, body, 503)
	detail := request("GET", "/products/"+id, "", 200)
	product := detail["product"].(map[string]any)
	if product["sold"] != false || product["sale"] != nil || detail["available"] != true || detail["seller"].(map[string]any)["completedSales"] != float64(0) {
		t.Fatalf("partial commit: %v", detail)
	}
	saleCount(t, db, "transactions", bson.M{}, 0)
	saleCount(t, db, "listings", bson.M{"_id": id}, 1)
	t.Log("persisted rollback evidence: archive unsold, sale absent, eligibility retained, transactions=0, completedSales=0")
	if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "transactions"}, {Key: "validator", Value: bson.M{}}}).Err(); err != nil {
		t.Fatal(err)
	}
	authRequest(t, base, "POST", "/products/"+id+"/sale", token, body, 200)
	saleCount(t, db, "transactions", bson.M{"success": true}, 1)
}

func TestSaleOpenAPIContract(t *testing.T) {
	_, request := setup(t, time.Hour)
	spec := request("GET", "/openapi.json", "", 200)
	path, ok := spec["paths"].(map[string]any)["/products/{id}/sale"].(map[string]any)
	if !ok {
		t.Fatal("sale operation missing")
	}
	op := path["post"].(map[string]any)
	if len(op["security"].([]any)) != 1 {
		t.Fatal("sale authentication undocumented")
	}
	responses := op["responses"].(map[string]any)
	for _, status := range []string{"200", "400", "401", "403", "404", "409", "503"} {
		if responses[status] == nil {
			t.Fatalf("missing %s response", status)
		}
	}
}

func TestSaleConcurrentKeyConflictRollsBackOtherProduct(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	ids := []string{}
	for i := 0; i < 2; i++ {
		ids = append(ids, authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)["id"].(string))
	}
	results := make(chan saleHTTPResult, 2)
	start := make(chan struct{})
	for _, id := range ids {
		go func(id string) {
			<-start
			results <- sendSaleRequest(base, id, token, `{"buyerId":"buyer","priceCents":42,"idempotencyKey":"shared-products"}`)
		}(id)
	}
	close(start)
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		switch r.status {
		case 200:
			successes++
		case 409:
			conflicts++
			if !strings.Contains(r.body, "idempotency_conflict") {
				t.Fatal(r.body)
			}
			detail := request("GET", "/products/"+r.id, "", 200)
			if detail["available"] != true || detail["product"].(map[string]any)["sale"] != nil {
				t.Fatalf("key conflict partially committed: %v", detail)
			}
		default:
			t.Fatalf("status=%d %s", r.status, r.body)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	saleCount(t, db, "transactions", bson.M{}, 1)
	saleCount(t, db, "listings", bson.M{}, 1)
	if authRequest(t, base, "GET", "/users/me", token, "", 200)["completedSales"] != float64(1) {
		t.Fatal("duplicate seller count")
	}
}

// Each concurrent test owns its barrier and assertions; only HTTP transport is shared.
type saleHTTPResult struct {
	id     string
	status int
	body   string
	err    error
}

func sendSaleRequest(base, id, token, body string) saleHTTPResult {
	req, err := http.NewRequest("POST", base+"/products/"+id+"/sale", strings.NewReader(body))
	if err != nil {
		return saleHTTPResult{err: err}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return saleHTTPResult{err: err}
	}
	defer resp.Body.Close()
	contents, err := io.ReadAll(resp.Body)
	return saleHTTPResult{id: id, status: resp.StatusCode, body: string(contents), err: err}
}
