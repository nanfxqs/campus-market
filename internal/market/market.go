// Package market exposes the V1 HTTP API backed by MongoDB.
package market

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type Profile struct {
	ID             string `bson:"_id" json:"id"`
	Username       string `bson:"username" json:"username"`
	Nickname       string `bson:"nickname" json:"nickname"`
	Avatar         string `bson:"avatar" json:"avatar"`
	CreditScore    int    `bson:"creditScore" json:"creditScore"`
	CompletedSales int    `bson:"completedSales" json:"completedSales"`
	PasswordHash   string `bson:"passwordHash" json:"-"`
}
type session struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"userId"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

// Seed adds two demo accounts only to an empty, explicitly named database.
// It never drops or overwrites existing data. Full-scale seeds belong to #12.
func Seed(ctx context.Context, db *mongo.Database) error {
	names, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return err
	}
	for _, name := range names {
		n, err := db.Collection(name).CountDocuments(ctx, bson.M{})
		if err != nil {
			return err
		}
		if n != 0 {
			return errors.New("seed requires an empty database")
		}
	}
	_, err = db.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		return err
	}
	_, err = db.Collection("sessions").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)})
	if err != nil {
		return err
	}
	accounts := make([]any, 0, 2)
	for _, name := range []string{"seller", "buyer"} {
		hash, err := bcrypt.GenerateFromPassword([]byte("CampusDemo123!"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		accounts = append(accounts, Profile{ID: name, Username: name, Nickname: name, Avatar: "https://example.com/" + name + ".png", CreditScore: 100, PasswordHash: string(hash)})
	}
	_, err = db.Collection("users").InsertMany(ctx, accounts)
	if err != nil {
		return err
	}
	return Initialize(ctx, db)
}

func fail(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code}})
}
func decode(c *gin.Context, value any) bool {
	return decodeLimit(c, value, 4096)
}
func decodeLimit(c *gin.Context, value any, limit int64) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		fail(c, 400, "invalid_input")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		fail(c, 400, "invalid_input")
		return false
	}
	return true
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// New creates an API with a fixed access-token lifetime. Expiry is checked on
// every authenticated request independently of MongoDB's delayed TTL cleanup.
func New(db *mongo.Database, ttl time.Duration) http.Handler {
	return newAPI(db, ttl, time.Now)
}

func newAPI(db *mongo.Database, ttl time.Duration, confirmationTime func() time.Time) http.Handler {
	if ttl <= 0 {
		panic("access token lifetime must be positive")
	}
	router := gin.New()
	router.Use(gin.Recovery(), saleRequestLog)
	router.GET("/health", func(c *gin.Context) {
		if err := db.Client().Ping(c.Request.Context(), nil); err != nil {
			fail(c, 503, "unavailable")
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.POST("/auth/login", func(c *gin.Context) {
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !decode(c, &input) {
			return
		}
		if len(input.Username) < 1 || len(input.Username) > 64 || len(input.Password) < 1 || len(input.Password) > 72 {
			fail(c, 400, "invalid_input")
			return
		}
		var user Profile
		err := db.Collection("users").FindOne(c.Request.Context(), bson.M{"username": input.Username}).Decode(&user)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			fail(c, 503, "unavailable")
			return
		}
		// A valid dummy hash keeps unknown-account rejection on the bcrypt path.
		hash := user.PasswordHash
		if errors.Is(err, mongo.ErrNoDocuments) {
			hash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil || err != nil {
			fail(c, 401, "invalid_credentials")
			return
		}
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			fail(c, 503, "unavailable")
			return
		}
		token := hex.EncodeToString(raw)
		expires := time.Now().Add(ttl)
		if _, err := db.Collection("sessions").InsertOne(c.Request.Context(), session{ID: tokenHash(token), UserID: user.ID, ExpiresAt: expires}); err != nil {
			fail(c, 503, "unavailable")
			return
		}
		c.JSON(200, gin.H{"accessToken": token, "tokenType": "Bearer", "expiresIn": int64(ttl / time.Second), "expiresAt": expires.UTC()})
	})
	authorized := router.Group("")
	authorized.Use(func(c *gin.Context) {
		header := strings.Fields(c.GetHeader("Authorization"))
		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") || len(header[1]) != 64 {
			fail(c, 401, "unauthorized")
			return
		}
		var s session
		err := db.Collection("sessions").FindOne(c.Request.Context(), bson.M{"_id": tokenHash(header[1]), "expiresAt": bson.M{"$gt": time.Now()}}).Decode(&s)
		if errors.Is(err, mongo.ErrNoDocuments) {
			fail(c, 401, "unauthorized")
			return
		}
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		c.Set("userID", s.UserID)
		c.Next()
	})
	authorized.GET("/users/me", func(c *gin.Context) {
		var user Profile
		err := db.Collection("users").FindOne(c.Request.Context(), bson.M{"_id": c.GetString("userID")}).Decode(&user)
		if errors.Is(err, mongo.ErrNoDocuments) {
			fail(c, 401, "unauthorized")
			return
		}
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		c.JSON(200, user)
	})
	authorized.PATCH("/users/me", func(c *gin.Context) {
		var input struct {
			Nickname json.RawMessage `json:"nickname"`
			Avatar   json.RawMessage `json:"avatar"`
		}
		if !decode(c, &input) {
			return
		}
		changes := bson.M{}
		if input.Nickname != nil {
			var s string
			if string(input.Nickname) == "null" || json.Unmarshal(input.Nickname, &s) != nil {
				fail(c, 400, "invalid_input")
				return
			}
			if !utf8.ValidString(s) || utf8.RuneCountInString(s) < 1 || utf8.RuneCountInString(s) > 40 || strings.TrimSpace(s) == "" {
				fail(c, 400, "invalid_input")
				return
			}
			changes["nickname"] = s
		}
		if input.Avatar != nil {
			var s string
			if string(input.Avatar) == "null" || json.Unmarshal(input.Avatar, &s) != nil {
				fail(c, 400, "invalid_input")
				return
			}
			u, err := url.Parse(s)
			if err != nil || len(s) > 2048 || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
				fail(c, 400, "invalid_input")
				return
			}
			changes["avatar"] = s
		}
		if len(changes) == 0 {
			fail(c, 400, "invalid_input")
			return
		}
		var user Profile
		err := db.Collection("users").FindOneAndUpdate(c.Request.Context(), bson.M{"_id": c.GetString("userID")}, bson.M{"$set": changes}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&user)
		if errors.Is(err, mongo.ErrNoDocuments) {
			fail(c, 401, "unauthorized")
			return
		}
		if err != nil {
			fail(c, 503, "unavailable")
			return
		}
		c.JSON(200, user)
	})
	productRoutes(router, authorized, db)
	saleRoutes(authorized, db, confirmationTime)
	router.GET("/search", func(c *gin.Context) { searchPage(c, db) })
	priceRoutes(authorized, db)
	router.GET("/openapi.json", func(c *gin.Context) { c.Data(200, "application/json", openAPI) })
	router.GET("/docs", func(c *gin.Context) { c.Data(200, "text/html; charset=utf-8", []byte(swaggerHTML)) })
	return router
}
