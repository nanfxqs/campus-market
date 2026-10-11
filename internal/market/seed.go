package market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SeedOptions identifies a reproducible experiment. BaseTime is deliberately
// independent of the wall clock; TTL and HTTP eligibility still use real time.
type SeedOptions struct {
	Mode         string
	RandomSeed   int64
	BaseTime     time.Time
	Reset        bool
	ManifestPath string
}

// Public demonstration credentials only; fixed salt makes restores reproducible.
const seedPasswordHash = "$2a$10$6PUS9DLqLoe6KbHNsgtywu8Ss2nniPO7tSx.0aYSp44.LGgFcTpN."

// SeedCounts describes the imported dataset, not a performance-test result.
type SeedCounts struct {
	Users        int `bson:"users" json:"users"`
	Active       int `bson:"active" json:"active"`
	Sold         int `bson:"sold" json:"sold"`
	Transactions int `bson:"transactions" json:"transactions"`
	PriceChanges int `bson:"priceChanges" json:"priceChanges"`
}

// SeedDataset refuses any existing data unless reset explicitly targets db.
// Imports are batched and not globally atomic; a failed import is intentionally
// nonempty and must be explicitly reset rather than silently resumed.
func SeedDataset(ctx context.Context, db *mongo.Database, o SeedOptions) (SeedCounts, error) {
	counts := SeedCounts{}
	switch o.Mode {
	case "v2":
		counts = SeedCounts{10000, 50000, 180000, 300000, 575000}
	case "full":
		counts = SeedCounts{10000, 20000, 180000, 300000, 500000}
	case "small":
		counts = SeedCounts{100, 200, 1800, 3000, 5000}
	case "stress":
		counts = SeedCounts{10000, 30000, 0, 0, 75000}
	case "demo":
		counts = SeedCounts{Users: 2}
	default:
		return counts, errors.New("mode must be v2, full, small, stress or demo")
	}
	name := db.Name()
	if name == "" || strings.EqualFold(name, "admin") || strings.EqualFold(name, "local") || strings.EqualFold(name, "config") {
		return counts, errors.New("an explicitly named non-system experiment database is required")
	}
	if o.BaseTime.IsZero() || o.BaseTime.Year() < 1971 || o.BaseTime.Year() > 9998 {
		return counts, errors.New("base time must be RFC3339 in years 1971..9998")
	}
	o.BaseTime = o.BaseTime.UTC().Truncate(time.Millisecond)
	if o.Mode == "v2" && o.BaseTime.Add(35*24*time.Hour).Before(time.Now().Add(24*time.Hour)) {
		return counts, errors.New("V2 base time must leave at least 24 hours of valid load eligibility")
	}
	if o.Mode == "v2" && o.BaseTime.After(time.Now()) {
		return counts, errors.New("V2 base time cannot be in the future")
	}
	if o.ManifestPath != "" && o.Mode != "v2" {
		return counts, errors.New("--manifest requires mode v2")
	}
	if o.Reset {
		if err := db.Drop(ctx); err != nil {
			return counts, err
		}
	} else {
		names, err := db.ListCollectionNames(ctx, bson.M{})
		if err != nil {
			return counts, err
		}
		for _, name := range names {
			n, err := db.Collection(name).CountDocuments(ctx, bson.M{}, options.Count().SetLimit(1))
			if err != nil {
				return counts, err
			}
			if n > 0 {
				return counts, errors.New("seed requires an empty database; use --reset to rebuild only the specified DB_NAME")
			}
		}
	}
	if o.Mode == "demo" {
		return counts, Seed(ctx, db)
	}
	if err := Initialize(ctx, db); err != nil {
		return counts, err
	}
	if _, err := db.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetUnique(true)}); err != nil {
		return counts, err
	}
	if _, err := db.Collection("sessions").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)}); err != nil {
		return counts, err
	}
	// V2 uses unchanged V1 background IDs and PRNG sequence, then appends its
	// dedicated cohort. Refuse an expired experiment before any import work.
	rng := rand.New(rand.NewSource(o.RandomSeed))
	users := make([]Profile, counts.Users)
	userID := func(i int) string {
		if i == 0 {
			return "seller"
		}
		if i == 1 {
			return "buyer"
		}
		return fmt.Sprintf("user-%05d", i)
	}
	for i := range users {
		id := userID(i)
		users[i] = Profile{ID: id, Username: id, Nickname: fmt.Sprintf("校园卖家%d（已更新）", i), Avatar: fmt.Sprintf("https://example.com/avatars/current-%d.png", i), CreditScore: 60 + rng.Intn(41), PasswordHash: seedPasswordHash}
	}
	writer := seedWriter{ctx: ctx, db: db, batches: map[string][]any{}}
	backgroundActive := counts.Active
	if o.Mode == "v2" {
		backgroundActive = 20000
	}
	manifest := SeedManifest{Version: 1, Database: name, Mode: o.Mode, RandomSeed: o.RandomSeed, BaseTime: o.BaseTime, Counts: counts}
	for i := 0; i < counts.Active+counts.Sold; i++ {
		id := fmt.Sprintf("%024x", i+1)
		seller := i % counts.Users
		sold := i >= backgroundActive && i < backgroundActive+counts.Sold
		dedicated := o.Mode == "v2" && i >= backgroundActive+counts.Sold
 if dedicated { seller = i % 50 }
		published := o.BaseTime.Add(-time.Duration(1+rng.Intn(25)) * 24 * time.Hour)
		if sold {
			published = o.BaseTime.Add(-time.Duration(40+rng.Intn(15)) * 24 * time.Hour)
		}
		p := Product{ID: id, SellerID: userID(seller), PublishedAt: published, ExpiresAt: published.Add(1440 * time.Hour), Sold: sold, InitialPriceCents: int64(10000 + rng.Intn(100000))}
		p.PriceCents = p.InitialPriceCents
		p.Condition = []string{"全新", "几乎全新", "轻度使用", "明显使用"}[i%4]
		switch i % 3 {
		case 0:
			p.CategoryID = "textbooks"
			p.Title = "高等数学教材"
			p.Description = "校园二手课本，附笔记"
			p.Attributes = map[string]any{"author": "同济大学", "edition": 2, "isbn": "9787040396638", "notes": true}
		case 1:
			p.CategoryID = "bicycles"
			p.Title = []string{"自行车", "单车", "脚踏车"}[(i/3)%3] + " 校园通勤"
			p.Description = "自行车／单车／脚踏车，轻便耐用"
			p.Attributes = map[string]any{"brand": "永久", "wheelSize": 26, "foldable": i%2 == 0, "colors": []string{"蓝色", "白色"}}
		case 2:
			p.CategoryID = "electronics"
			p.Title = "数码平板电脑"
			p.Description = "上课学习使用，附充电器"
			p.Attributes = map[string]any{"brand": "校园数码", "model": "学习版", "storageGB": 128, "accessories": []string{"充电器", "保护套"}}
		}
		if i%17 == 0 {
			p.Description += "，几乎全心，便宜转让（错别字案例）"
		}
		for j := 0; j < 1+i%9; j++ {
			p.Images = append(p.Images, fmt.Sprintf("https://example.com/products/%s/%d.jpg", id, j))
		}
		for j := 0; j < 2+i%2; j++ {
			next := p.PriceCents - int64(100+rng.Intn(500))
			change := PriceChange{ID: fmt.Sprintf("%024x", i*9+j+1), ProductID: id, OldPriceCents: p.PriceCents, NewPriceCents: next, ChangedAt: published.Add(time.Duration(j+1) * time.Hour)}
			p.PriceCents = next
			if err := writer.add("priceChanges", change); err != nil {
				return counts, err
			}
		}
		if sold {
			users[seller].CompletedSales++
			at := published.Add(24 * time.Hour)
			p.Sale = &Sale{BuyerID: userID((seller + 1) % counts.Users), PriceCents: p.PriceCents - 50, ConfirmedAt: at}
			a := SaleAttempt{SellerID: p.SellerID, ProductID: id, BuyerID: p.Sale.BuyerID, PriceCents: p.Sale.PriceCents, IdempotencyKey: fmt.Sprintf("seed-sale-%d", i), Success: true, Code: "sold", ConfirmedAt: at}
			a.ID = saleAttemptID(a.SellerID, a.IdempotencyKey)
			if err := writer.add("transactions", a); err != nil {
				return counts, err
			}
		} else {
			if err := writer.add("listings", bson.M{"_id": id, "expiresAt": p.ExpiresAt}); err != nil {
				return counts, err
			}
		}
		doc := bson.M{}
		encoded, err := bson.Marshal(p)
		if err != nil {
			return counts, err
		}
		if err := bson.Unmarshal(encoded, &doc); err != nil {
			return counts, err
		}
		doc["contentVersion"] = int64(1)
		if o.Mode == "v2" {
			doc["seedCohort"] = "background"
			if dedicated {
				doc["seedCohort"] = "dedicated"
				manifest.Products = append(manifest.Products, SeedManifestProduct{ProductID: id, SellerID: p.SellerID, BuyerID: userID((seller + 1) % counts.Users), PriceCents: p.PriceCents, ExpiresAt: p.ExpiresAt, IdempotencyKey: "v2-sale-" + id})
			}
		}
		if err := writer.add("products", doc); err != nil {
			return counts, err
		}
	}
	for i := counts.Sold; i < counts.Transactions; i++ {
		product := backgroundActive + (i-counts.Sold)%counts.Sold
		seller := product % counts.Users
		a := SaleAttempt{SellerID: userID(seller), ProductID: fmt.Sprintf("%024x", product+1), BuyerID: userID((seller + 1) % counts.Users), PriceCents: 10000, IdempotencyKey: fmt.Sprintf("seed-failure-%d", i), Code: "product_sold", ConfirmedAt: o.BaseTime.Add(-time.Hour)}
		a.ID = saleAttemptID(a.SellerID, a.IdempotencyKey)
		if err := writer.add("transactions", a); err != nil {
			return counts, err
		}
	}
	for _, u := range users {
		manifest.Accounts = append(manifest.Accounts, SeedManifestAccount{ID: u.ID, Username: u.Username, Password: "CampusDemo123!", CompletedSales: u.CompletedSales})
		if err := writer.add("users", u); err != nil {
			return counts, err
		}
		if err := writer.add("profileChanges", bson.M{"_id": u.ID, "userId": u.ID, "oldNickname": u.Username, "newNickname": u.Nickname, "oldAvatar": "https://example.com/avatars/original.png", "newAvatar": u.Avatar, "changedAt": o.BaseTime.Add(-30 * 24 * time.Hour)}); err != nil {
			return counts, err
		}
	}
	// Historical textbook edition rules were strings; current rules are numbers.
	// Archives intentionally retain old values, while active data uses current rules.
	if err := writer.flush(); err != nil {
		return counts, err
	}
	// Flush before applying evolution to include the final batch.
	if _, err := db.Collection("products").UpdateMany(ctx, bson.M{"sold": true, "categoryId": "textbooks"}, bson.M{"$set": bson.M{"attributes.edition": "第一版（旧规则）"}}); err != nil {
		return counts, err
	}
	if _, err := db.Collection("categoryChanges").InsertOne(ctx, bson.M{"_id": "textbooks-v2", "categoryId": "textbooks", "attribute": "edition", "oldType": "string", "newType": "number", "changedAt": o.BaseTime.Add(-30 * 24 * time.Hour)}); err != nil {
		return counts, err
	}
	_, err := db.Collection("seed_metadata").InsertOne(ctx, bson.M{"_id": "dataset", "version": 1, "mode": o.Mode, "randomSeed": o.RandomSeed, "baseTime": o.BaseTime, "counts": counts, "synonymsVersion": "bicycles_v1"})
	if err != nil {
		return counts, err
	}
	if o.ManifestPath != "" {
		data, err := json.Marshal(manifest)
		if err != nil {
			return counts, err
		}
		if err := os.WriteFile(o.ManifestPath, append(data, '\n'), 0644); err != nil {
			return counts, err
		}
	}
	return counts, nil
}

// SeedManifest contains only designated load products and the registered accounts
// they reuse. Credentials are deliberately public synthetic experiment accounts.
type SeedManifest struct {
	Version    int                   `json:"version"`
	Database   string                `json:"database"`
	Mode       string                `json:"mode"`
	RandomSeed int64                 `json:"randomSeed"`
	BaseTime   time.Time             `json:"baseTime"`
	Counts     SeedCounts            `json:"counts"`
	Accounts   []SeedManifestAccount `json:"accounts"`
	Products   []SeedManifestProduct `json:"products"`
}
type SeedManifestAccount struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	CompletedSales int    `json:"completedSales"`
}
type SeedManifestProduct struct {
	ProductID      string    `json:"productId"`
	SellerID       string    `json:"sellerId"`
	BuyerID        string    `json:"buyerId"`
	PriceCents     int64     `json:"priceCents"`
	ExpiresAt      time.Time `json:"expiresAt"`
	IdempotencyKey string    `json:"idempotencyKey"`
}

// A bounded batch keeps the full dataset import memory independent of its size.
type seedWriter struct {
	ctx     context.Context
	db      *mongo.Database
	batches map[string][]any
}

func (w *seedWriter) add(name string, doc any) error {
	w.batches[name] = append(w.batches[name], doc)
	if len(w.batches[name]) >= 1000 {
		return w.flushCollection(name)
	}
	return nil
}
func (w *seedWriter) flushCollection(name string) error {
	if len(w.batches[name]) == 0 {
		return nil
	}
	_, err := w.db.Collection(name).InsertMany(w.ctx, w.batches[name])
	w.batches[name] = nil
	return err
}
func (w *seedWriter) flush() error {
	for name := range w.batches {
		if err := w.flushCollection(name); err != nil {
			return err
		}
	}
	return nil
}
