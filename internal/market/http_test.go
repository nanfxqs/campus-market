package market_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nanfxqs/campus-market/internal/market"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func setup(t *testing.T, ttl time.Duration) (string, func(string, string, string, int) map[string]any) {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Fatal("MONGO_URI required: run docker compose run --rm verify test ./internal/market")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	name := "campus_auth_test_" + time.Now().Format("150405000000000")
	db := client.Database(name)
	t.Cleanup(func() { db.Drop(context.Background()); client.Disconnect(context.Background()) })
	command := exec.Command("go", "run", "../../cmd/market", "seed")
	command.Env = append(os.Environ(), "DB_NAME="+name)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("seed: %s %v", out, err)
	}
	server := httptest.NewServer(market.New(db, ttl))
	t.Cleanup(server.Close)
	request := func(method, path, body string, want int) map[string]any {
		t.Helper()
		return authRequest(t, server.URL, method, path, "", body, want)
	}
	return server.URL, request
}
func TestSeedLogin(t *testing.T) {
	_, request := setup(t, time.Hour)
	request("POST", "/auth/login", `{"username":"seller","password":"wrong"}`, 401)
	request("POST", "/auth/login", `{"username":"unknown","password":"CampusDemo123!"}`, 401)
	result := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)
	if result["expiresIn"] != float64(3600) || result["tokenType"] != "Bearer" || result["accessToken"] == "" {
		t.Fatalf("invalid login response: %v", result)
	}
}

func authRequest(t *testing.T, base, method, path, token, body string, want int) map[string]any {
	t.Helper()
	req, _ := http.NewRequest(method, base+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s: status=%d want=%d body=%v", method, path, resp.StatusCode, want, result)
	}
	return result
}
func TestOwnProfileAndIdentityProtection(t *testing.T) {
	base, request := setup(t, time.Hour)
	seller := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	buyer := request("POST", "/auth/login", `{"username":"buyer","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	authRequest(t, base, "GET", "/users/me", "", "", 401)
	authRequest(t, base, "PATCH", "/users/me", "", `{"nickname":"冒用"}`, 401)
	authRequest(t, base, "GET", "/users/me", seller[:63]+"z", "", 401)
	for _, body := range []string{`{"nickname":"冒用","id":"buyer"}`, `{"nickname":"冒用","sellerId":"buyer"}`, `{"creditScore":999}`, `{}`, `{"nickname":" "}`, `{"nickname":"` + string(bytes.Repeat([]byte("a"), 41)) + `"}`, `{"avatar":"file:///tmp/photo"}`, `{"avatar":"https://user:pass@example.com/a"}`, `{"nickname":null}`, `{"nickname":null,"avatar":"https://example.com/a"}`, `{"nickname":"ok"} {}`} {
		authRequest(t, base, "PATCH", "/users/me", seller, body, 400)
	}
	changed := authRequest(t, base, "PATCH", "/users/me", seller, `{"nickname":"校园卖家","avatar":"https://example.com/current.png"}`, 200)
	if changed["id"] != "seller" || changed["nickname"] != "校园卖家" || changed["avatar"] != "https://example.com/current.png" || changed["creditScore"] != float64(100) {
		t.Fatalf("profile: %v", changed)
	}
	current := authRequest(t, base, "GET", "/users/me?userId=buyer", seller, "", 200)
	if current["nickname"] != "校园卖家" || current["id"] != "seller" {
		t.Fatalf("persisted profile: %v", current)
	}
	other := authRequest(t, base, "GET", "/users/me", buyer, "", 200)
	if other["nickname"] != "buyer" || other["creditScore"] != float64(100) {
		t.Fatalf("other user changed: %v", other)
	}
	if _, ok := current["passwordHash"]; ok {
		t.Fatal("password hash leaked")
	}
}
func TestExpiredAccessToken(t *testing.T) {
	base, request := setup(t, 100*time.Millisecond)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	authRequest(t, base, "GET", "/users/me", token, "", 200)
	time.Sleep(150 * time.Millisecond)
	authRequest(t, base, "GET", "/users/me", token, "", 401)
	authRequest(t, base, "PATCH", "/users/me", token, `{"nickname":"expired"}`, 401)
}

func TestOpenAPIAndSwagger(t *testing.T) {
	base, request := setup(t, time.Hour)
	spec := request("GET", "/openapi.json", "", 200)
	if spec["openapi"] != "3.0.3" {
		t.Fatalf("OpenAPI: %v", spec)
	}
	paths := spec["paths"].(map[string]any)
	patch := paths["/users/me"].(map[string]any)["patch"].(map[string]any)
	if len(patch["security"].([]any)) != 1 {
		t.Fatal("profile auth undocumented")
	}
	resp, err := http.Get(base + "/docs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	contents, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 || !bytes.Contains(contents, []byte("SwaggerUIBundle")) || !bytes.Contains(contents, []byte("/openapi.json")) {
		t.Fatal("Swagger UI unavailable")
	}
}
func TestSeedCLIRefusesNonemptyDatabaseAndHashesCredentials(t *testing.T) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Fatal("MONGO_URI required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	name := "campus_seed_test_" + time.Now().Format("150405000000000")
	db := client.Database(name)
	t.Cleanup(func() { db.Drop(context.Background()); client.Disconnect(context.Background()) })
	seed := func(name string) error {
		cmd := exec.Command("go", "run", "../../cmd/market", "seed")
		cmd.Env = append(os.Environ(), "DB_NAME="+name)
		out, err := cmd.CombinedOutput()
		t.Logf("seed output: %s", out)
		return err
	}
	if err := seed(name); err != nil {
		t.Fatal(err)
	}
	if err := seed(name); err == nil {
		t.Fatal("nonempty database accepted")
	}
	if err := seed("admin"); err == nil {
		t.Fatal("system database accepted")
	}
	cursor, err := db.Collection("users").Find(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close(ctx)
	var rows []struct {
		Hash string `bson:"passwordHash"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Hash == rows[1].Hash {
		t.Fatal("expected two independently salted account hashes")
	}
	for _, row := range rows {
		if bcrypt.CompareHashAndPassword([]byte(row.Hash), []byte("CampusDemo123!")) != nil {
			t.Fatal("invalid password hash")
		}
	}
}
