package market_test

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"testing"
	"time"
)

func TestSellerChangesPriceWithoutExtendingEligibility(t *testing.T) {
	base, request := setup(t, time.Hour)
	seller := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	buyer := request("POST", "/auth/login", `{"username":"buyer","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	p := authRequest(t, base, "POST", "/products", seller, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)
	path := "/products/" + p["id"].(string)
	authRequest(t, base, "PATCH", path+"/price", "", `{"priceCents":12000}`, 401)
	authRequest(t, base, "PATCH", path+"/price", buyer, `{"priceCents":12000}`, 403)
	before := time.Now().Add(-time.Second)
	changed := authRequest(t, base, "PATCH", path+"/price", seller, `{"priceCents":12000}`, 200)
	if changed["priceCents"] != float64(12000) || changed["initialPriceCents"] != float64(12345) || changed["expiresAt"] != p["expiresAt"] {
		t.Fatalf("price or deadline: %v", changed)
	}
	authRequest(t, base, "PATCH", path+"/price", seller, `{"priceCents":12000}`, 200)
	detail := request("GET", path, "", 200)
	history := detail["priceHistory"].([]any)
	if len(history) != 1 {
		t.Fatalf("same price created history: %v", history)
	}
	row := history[0].(map[string]any)
	stamp, err := time.Parse(time.RFC3339Nano, row["changedAt"].(string))
	if err != nil || stamp.Before(before) || stamp.After(time.Now()) || row["oldPriceCents"] != float64(12345) || row["newPriceCents"] != float64(12000) {
		t.Fatalf("history: %v", row)
	}
}

func TestPriceHistoryThroughChangesAndUnavailableArchives(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	seller := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	p := authRequest(t, base, "POST", "/products", seller, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)
	id := p["id"].(string)
	path := "/products/" + id
	for _, body := range []string{`{}`, `{"priceCents":null}`, `{"priceCents":0}`, `{"priceCents":-1}`, `{"priceCents":1.5}`, `{"priceCents":1000000000001}`, `{"priceCents":100,"sellerId":"buyer"}`} {
		authRequest(t, base, "PATCH", path+"/price", seller, body, 400)
	}
	for _, price := range []int{12000, 11000, 10000, 9000, 8000, 7000, 6000} {
		authRequest(t, base, "PATCH", path+"/price", seller, fmt.Sprintf(`{"priceCents":%d}`, price), 200)
	}
	detail := request("GET", path, "", 200)
	if len(detail["priceHistory"].([]any)) != 5 {
		t.Fatal(detail)
	}
	cursor := ""
	expected := []float64{6000, 7000, 8000, 9000, 10000, 11000, 12000}
	expectedOld := []float64{7000, 8000, 9000, 10000, 11000, 12000, 12345}
	rows := []any{}
	for page := 0; page < 4; page++ {
		url := path + "/price-history?limit=2"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		history := request("GET", url, "", 200)
		rows = append(rows, history["items"].([]any)...)
		cursor = history["nextCursor"].(string)
		if cursor == "" {
			break
		}
	}
	if len(rows) != 7 || cursor != "" {
		t.Fatalf("incomplete history: %v", rows)
	}
	for i, raw := range rows {
		row := raw.(map[string]any)
		if row["newPriceCents"] != expected[i] || row["oldPriceCents"] != expectedOld[i] {
			t.Fatal(row)
		}
	}
	ctx := context.Background()
	for _, state := range []string{"expired", "delisted", "sold"} {
		t.Run(state, func(t *testing.T) {
			item := authRequest(t, base, "POST", "/products", seller, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)
			itemPath := "/products/" + item["id"].(string)
			authRequest(t, base, "PATCH", itemPath+"/price", seller, `{"priceCents":10000}`, 200)
			var err error
			switch state {
			case "expired":
				_, err = db.Collection("products").UpdateOne(ctx, bson.M{"_id": item["id"]}, bson.M{"$set": bson.M{"expiresAt": time.Now().Add(-time.Second)}})
			case "delisted":
				_, err = db.Collection("listings").DeleteOne(ctx, bson.M{"_id": item["id"]})
			case "sold":
				_, err = db.Collection("products").UpdateOne(ctx, bson.M{"_id": item["id"]}, bson.M{"$set": bson.M{"sold": true}})
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, price := range []int{10000, 9000} {
				authRequest(t, base, "PATCH", itemPath+"/price", seller, fmt.Sprintf(`{"priceCents":%d}`, price), 409)
			}
			archived := request("GET", itemPath, "", 200)
			if archived["product"].(map[string]any)["priceCents"] != float64(10000) || len(archived["priceHistory"].([]any)) != 1 {
				t.Fatal(archived)
			}
			history := request("GET", itemPath+"/price-history", "", 200)
			if len(history["items"].([]any)) != 1 {
				t.Fatal(history)
			}
		})
	}
}

func TestPriceHistoryFailureRollsBackCurrentPrice(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	seller := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	p := authRequest(t, base, "POST", "/products", seller, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)
	path := "/products/" + p["id"].(string)
	ctx := context.Background()
	if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "priceChanges"}, {Key: "validator", Value: bson.M{"blocked": bson.M{"$exists": true}}}, {Key: "validationLevel", Value: "strict"}}).Err(); err != nil {
		t.Fatal(err)
	}
	authRequest(t, base, "PATCH", path+"/price", seller, `{"priceCents":10000}`, 503)
	detail := request("GET", path, "", 200)
	if detail["product"].(map[string]any)["priceCents"] != float64(12345) || len(detail["priceHistory"].([]any)) != 0 {
		t.Fatal(detail)
	}
	if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "priceChanges"}, {Key: "validator", Value: bson.M{}}}).Err(); err != nil {
		t.Fatal(err)
	}
	authRequest(t, base, "PATCH", path+"/price", seller, `{"priceCents":10000}`, 200)
}

func TestPriceSwaggerContract(t *testing.T) {
	_, request := setup(t, time.Hour)
	spec := request("GET", "/openapi.json", "", 200)
	operation := spec["paths"].(map[string]any)["/products/{id}/price"].(map[string]any)["patch"].(map[string]any)
	if len(operation["security"].([]any)) != 1 {
		t.Fatal("price authorization missing")
	}
	responses := operation["responses"].(map[string]any)
	for _, code := range []string{"200", "400", "401", "403", "404", "409", "503"} {
		if _, ok := responses[code]; !ok {
			t.Fatalf("missing status %s", code)
		}
	}
	schema := spec["components"].(map[string]any)["schemas"].(map[string]any)["ChangePrice"].(map[string]any)
	price := schema["properties"].(map[string]any)["priceCents"].(map[string]any)
	if price["type"] != "integer" || price["minimum"] != float64(1) || price["maximum"] != float64(1000000000000) || schema["additionalProperties"] != false {
		t.Fatal(schema)
	}
}
