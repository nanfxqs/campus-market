package market_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/nanfxqs/campus-market/internal/market"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestSeedCLIResetAndReproduction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	baseTime := time.Now().UTC().Truncate(24 * time.Hour)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("campus_seed_test_%d", time.Now().UnixNano())
	db := client.Database(name)
	other := client.Database(name + "_other")
	t.Cleanup(func() {
		db.Drop(context.Background())
		other.Drop(context.Background())
		client.Disconnect(context.Background())
	})
	if _, err := other.Collection("sentinel").InsertOne(ctx, bson.M{"_id": "keep"}); err != nil {
		t.Fatal(err)
	}
	run := func(wantOK bool, extra ...string) {
		t.Helper()
		args := append([]string{"run", "../../cmd/market", "seed", "--mode=small", "--base-time=" + baseTime.Format(time.RFC3339), "--random-seed=42"}, extra...)
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "DB_NAME="+name)
		out, err := cmd.CombinedOutput()
		if (err == nil) != wantOK {
			t.Fatalf("seed: %s %v", out, err)
		}
	}
	run(true)
	var metadata bson.M
	if err := db.Collection("seed_metadata").FindOne(ctx, bson.M{"_id": "dataset"}).Decode(&metadata); err != nil {
		t.Fatal(err)
	}
	if _, exists := metadata["password"]; exists {
		t.Fatal("seed metadata must not contain a plaintext password")
	}
	for collection, want := range map[string]int64{"users": 100, "products": 2000, "listings": 200, "transactions": 3000, "priceChanges": 5000} {
		n, err := db.Collection(collection).CountDocuments(ctx, bson.M{})
		if err != nil || n != want {
			t.Fatalf("%s count=%d want=%d err=%v", collection, n, want, err)
		}
	}
	snapshot := func() []bson.M {
		var all []bson.M
		for _, c := range []string{"users", "products", "listings", "transactions", "priceChanges", "seed_metadata", "categoryChanges", "profileChanges"} {
			cur, err := db.Collection(c).Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
			if err != nil {
				t.Fatal(err)
			}
			var rows []bson.M
			if err := cur.All(ctx, &rows); err != nil {
				t.Fatal(err)
			}
			all = append(all, bson.M{"collection": c, "rows": rows})
		}
		return all
	}
	before := snapshot()
	run(false)
	run(false, "--reset", "--mode=invalid")
	run(false, "--reset", "--base-time=0001-01-01T00:00:00Z")
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("invalid reset arguments modified database")
	}
	run(true, "--reset")
	after := snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("same seed and base time did not restore identical dataset")
	}
	if n, err := other.Collection("sentinel").CountDocuments(ctx, bson.M{}); err != nil || n != 1 {
		t.Fatalf("reset touched other database: %d %v", n, err)
	}
	verifySeedDataset(t, ctx, db)
	run(true, "--reset", "--random-seed=43")
	if reflect.DeepEqual(before, snapshot()) {
		t.Fatal("random seed did not change dataset")
	}
	run(true, "--reset", "--mode=stress")
	for c, want := range map[string]int64{"users": 10000, "products": 30000, "listings": 30000, "transactions": 0, "priceChanges": 75000} {
		n, err := db.Collection(c).CountDocuments(ctx, bson.M{})
		if err != nil || n != want {
			t.Fatalf("stress %s count=%d want=%d err=%v", c, n, want, err)
		}
	}
}

func verifySeedDataset(t *testing.T, ctx context.Context, db *mongo.Database) {
	t.Helper()
	var products []market.Product
	cur, err := db.Collection("products").Find(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cur.All(ctx, &products); err != nil {
		t.Fatal(err)
	}
	var histories []market.PriceChange
	cur, err = db.Collection("priceChanges").Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "changedAt", Value: 1}}))
	if err != nil {
		t.Fatal(err)
	}
	if err := cur.All(ctx, &histories); err != nil {
		t.Fatal(err)
	}
	history := map[string][]market.PriceChange{}
	for _, h := range histories {
		history[h.ProductID] = append(history[h.ProductID], h)
	}
	var attempts []market.SaleAttempt
	cur, err = db.Collection("transactions").Find(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cur.All(ctx, &attempts); err != nil {
		t.Fatal(err)
	}
	sales := map[string]market.SaleAttempt{}
	completed := map[string]int{}
	byID := map[string]market.Product{}
	for _, p := range products {
		byID[p.ID] = p
	}
	for _, a := range attempts {
		p, exists := byID[a.ProductID]
		if !exists || a.SellerID != p.SellerID || a.BuyerID == a.SellerID {
			t.Fatalf("invalid attempt %+v", a)
		}
		if a.Success {
			if _, duplicate := sales[p.ID]; duplicate {
				t.Fatal("duplicate successful transaction")
			}
			if !p.Sold || p.Sale == nil || p.Sale.BuyerID != a.BuyerID || p.Sale.PriceCents != a.PriceCents || !p.Sale.ConfirmedAt.Equal(a.ConfirmedAt) {
				t.Fatalf("sale mismatch %+v", a)
			}
			sales[p.ID] = a
			completed[a.SellerID]++
		} else if a.Code != "product_sold" || !p.Sold || !a.ConfirmedAt.After(p.Sale.ConfirmedAt) {
			t.Fatalf("invalid business failure %+v", a)
		}
	}
	images := map[int]bool{}
	conditions := map[string]bool{}
	categories := map[string]bool{}
	now := time.Now().UTC()
	for _, p := range products {
		images[len(p.Images)] = true
		conditions[p.Condition] = true
		categories[p.CategoryID] = true
		if p.PublishedAt.After(now) || (!p.Sold && !p.ExpiresAt.After(now)) {
			t.Fatalf("seed product must be published already and active products unexpired: %s", p.ID)
		}
		if len(p.Images) < 1 || len(p.Images) > 9 || p.ExpiresAt.Sub(p.PublishedAt) != 1440*time.Hour {
			t.Fatalf("invalid product %+v", p)
		}
		price := p.InitialPriceCents
		at := p.PublishedAt
		for _, h := range history[p.ID] {
			if h.OldPriceCents != price || h.NewPriceCents == price || h.NewPriceCents <= 0 || !h.ChangedAt.After(at) || !h.ChangedAt.Before(p.ExpiresAt) {
				t.Fatalf("broken price chain %+v", h)
			}
			price = h.NewPriceCents
			at = h.ChangedAt
		}
		if price != p.PriceCents || len(history[p.ID]) < 2 || len(history[p.ID]) > 3 {
			t.Fatalf("incomplete history %s", p.ID)
		}
		if p.Sold {
			if _, ok := sales[p.ID]; !ok || !p.Sale.ConfirmedAt.After(at) {
				t.Fatalf("missing or out-of-order sale %s", p.ID)
			}
		}
		n, err := db.Collection("listings").CountDocuments(ctx, bson.M{"_id": p.ID, "expiresAt": p.ExpiresAt})
		if err != nil || (p.Sold && n != 0) || (!p.Sold && n != 1) {
			t.Fatalf("eligibility mismatch %s %d %v", p.ID, n, err)
		}
	}
	if len(images) != 9 || len(conditions) != 4 || len(categories) != 3 {
		t.Fatal("missing dataset heterogeneity")
	}
	var users []market.Profile
	cur, err = db.Collection("users").Find(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cur.All(ctx, &users); err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.CompletedSales != completed[u.ID] {
			t.Fatalf("seller count mismatch %+v", u)
		}
	}
	for collection, filter := range map[string]bson.M{
		"profileChanges":  {},
		"categoryChanges": {"attribute": "edition", "oldType": "string", "newType": "number"},
		"synonyms_v1":     {"synonyms": bson.M{"$all": bson.A{"自行车", "单车", "脚踏车"}}},
		"products":        {"description": bson.M{"$regex": "全心"}},
	} {
		n, err := db.Collection(collection).CountDocuments(ctx, filter)
		if err != nil || n == 0 {
			t.Fatalf("missing %s cases: %v", collection, err)
		}
	}
	knownUsers := map[string]bool{}
	for _, u := range users {
		knownUsers[u.ID] = true
	}
	for _, a := range attempts {
		if !knownUsers[a.BuyerID] || !knownUsers[a.SellerID] {
			t.Fatalf("unregistered participant %+v", a)
		}
	}
	for _, p := range products {
		if p.CategoryID == "textbooks" {
			_, oldType := p.Attributes["edition"].(string)
			if oldType != p.Sold {
				t.Fatalf("category evolution mismatch %s", p.ID)
			}
		}
	}
	server := httptest.NewServer(market.New(db, time.Hour))
	defer server.Close()
	login := authRequest(t, server.URL, "POST", "/auth/login", "", `{"username":"seller","password":"CampusDemo123!"}`, 200)
	token := login["accessToken"].(string)
	detail := authRequest(t, server.URL, "GET", "/products/000000000000000000000001", "", "", 200)
	if detail["seller"] == nil {
		t.Fatal("seed detail must expose current seller")
	}
	// Seed attempts must participate in the real API's idempotency contract.
	for _, a := range attempts {
		if a.Success && a.SellerID == "seller" {
			body := fmt.Sprintf(`{"buyerId":%q,"priceCents":%d,"idempotencyKey":%q}`, a.BuyerID, a.PriceCents, a.IdempotencyKey)
			result := authRequest(t, server.URL, "POST", "/products/"+a.ProductID+"/sale", token, body, 200)
			if result["id"] != a.ID {
				t.Fatal("seed attempt replay did not return original result")
			}
			break
		}
	}
}
