package market_test

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"regexp"
	"strings"
	"testing"
	"time"
)

func productBody(category string, attributes map[string]any) map[string]any {
	return map[string]any{"categoryId": category, "title": "校园闲置", "description": "可线下查看", "priceCents": 12345, "condition": "轻度使用", "images": []string{"https://example.invalid/photo.png"}, "attributes": attributes}
}
func bodyJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
func TestPublishHeterogeneousProductsAndReadCurrentSeller(t *testing.T) {
	base, request := setup(t, time.Hour)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	categories := request("GET", "/categories", "", 200)["items"].([]any)
	if len(categories) != 3 {
		t.Fatalf("categories: %v", categories)
	}
	for _, category := range categories {
		row := category.(map[string]any)
		if row["name"] == "" || row["rules"] == nil || row["limits"] == nil {
			t.Fatalf("unreadable rules: %v", row)
		}
	}
	for _, example := range []struct {
		category   string
		attributes map[string]any
	}{
		{"textbooks", map[string]any{"author": "张三", "isbn": "9780000000001", "edition": 2, "notes": true, "tags": []string{"数学", "教材"}}},
		{"bicycles", map[string]any{"brand": "永久", "wheelSize": 26, "foldable": false}},
		{"electronics", map[string]any{"brand": "示例", "model": "X1", "storageGB": 256, "accessories": []string{"充电器"}}},
	} {
		t.Run(example.category, func(t *testing.T) {
			published := authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody(example.category, example.attributes)), 201)
			id := published["id"].(string)
			detail := request("GET", "/products/"+id, "", 200)
			p := detail["product"].(map[string]any)
			if p["sellerId"] != "seller" || p["categoryId"] != example.category || p["priceCents"] != float64(12345) || p["initialPriceCents"] != float64(12345) || p["condition"] != "轻度使用" || p["title"] != "校园闲置" || p["description"] != "可线下查看" || len(p["images"].([]any)) != 1 || bodyJSON(t, p["attributes"]) != bodyJSON(t, example.attributes) {
				t.Fatalf("incomplete product: %v", p)
			}
			start, err := time.Parse(time.RFC3339Nano, p["publishedAt"].(string))
			if err != nil {
				t.Fatal(err)
			}
			end, err := time.Parse(time.RFC3339Nano, p["expiresAt"].(string))
			if err != nil {
				t.Fatal(err)
			}
			if end.Sub(start) != 1440*time.Hour || time.Since(start) > time.Minute || detail["available"] != true {
				t.Fatalf("sale eligibility: %v", detail)
			}
			if len(detail["priceHistory"].([]any)) != 0 {
				t.Fatalf("initial price is not a price change: %v", detail)
			}
			history := request("GET", detail["priceHistoryUrl"].(string), "", 200)
			if len(history["items"].([]any)) != 0 {
				t.Fatalf("new product history: %v", history)
			}
			authRequest(t, base, "PATCH", "/users/me", token, `{"nickname":"最新卖家","avatar":"https://example.invalid/new.png"}`, 200)
			current := request("GET", "/products/"+id, "", 200)["seller"].(map[string]any)
			if current["nickname"] != "最新卖家" || current["avatar"] != "https://example.invalid/new.png" || current["creditScore"] != float64(100) {
				t.Fatalf("stale seller: %v", current)
			}
			if _, leaked := current["passwordHash"]; leaked {
				t.Fatal("password leaked")
			}
		})
	}
}

func TestPublishingValidatesCurrentCategoryRulesAndInputBoundaries(t *testing.T) {
	base, request := setup(t, time.Hour)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	for _, example := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"unknown category", func(p map[string]any) { p["categoryId"] = "other" }},
		{"required author", func(p map[string]any) { p["attributes"] = map[string]any{} }},
		{"author type", func(p map[string]any) { p["attributes"] = map[string]any{"author": 12} }},
		{"number type", func(p map[string]any) { p["attributes"] = map[string]any{"author": "A", "edition": "two"} }},
		{"boolean type", func(p map[string]any) {
			p["categoryId"] = "bicycles"
			p["attributes"] = map[string]any{"brand": "A", "wheelSize": 26, "foldable": "yes"}
		}},
		{"array type", func(p map[string]any) {
			p["categoryId"] = "electronics"
			p["attributes"] = map[string]any{"brand": "A", "model": "X", "accessories": []any{"充电器", 3}}
		}},
		{"nested object", func(p map[string]any) {
			p["attributes"] = map[string]any{"author": "A", "extra": map[string]any{"x": 1}}
		}},
		{"nested array", func(p map[string]any) { p["attributes"] = map[string]any{"author": "A", "extra": []any{[]any{1}}} }},
		{"null scalar", func(p map[string]any) { p["attributes"] = map[string]any{"author": "A", "extra": nil} }},
		{"null array element", func(p map[string]any) { p["attributes"] = map[string]any{"author": "A", "extra": []any{nil}} }},
		{"null attributes", func(p map[string]any) { p["attributes"] = nil }},
		{"too many extras", func(p map[string]any) {
			a := map[string]any{"author": "A"}
			for i := 0; i < 17; i++ {
				a[fmt.Sprintf("extra%d", i)] = true
			}
			p["attributes"] = a
		}},
		{"long attribute key", func(p map[string]any) { p["attributes"] = map[string]any{"author": "A", strings.Repeat("a", 41): 1} }},
		{"unsafe key", func(p map[string]any) { p["attributes"] = map[string]any{"author": "A", "bad.key": 1} }},
		{"long attribute string", func(p map[string]any) { p["attributes"] = map[string]any{"author": strings.Repeat("字", 257)} }},
		{"long array", func(p map[string]any) { p["attributes"] = map[string]any{"author": "A", "extra": make([]int, 17)} }},
		{"long string in array", func(p map[string]any) {
			p["attributes"] = map[string]any{"author": "A", "extra": []string{strings.Repeat("字", 257)}}
		}},
		{"zero images", func(p map[string]any) { p["images"] = []string{} }},
		{"null images", func(p map[string]any) { p["images"] = nil }},
		{"ten images", func(p map[string]any) { p["images"] = repeatedImages(10) }},
		{"file URL", func(p map[string]any) { p["images"] = []string{"file:///tmp/a.png"} }},
		{"FTP URL", func(p map[string]any) { p["images"] = []string{"ftp://example.com/a.png"} }},
		{"relative URL", func(p map[string]any) { p["images"] = []string{"/a.png"} }},
		{"missing hostname", func(p map[string]any) { p["images"] = []string{"https:///a.png"} }},
		{"URL credentials", func(p map[string]any) { p["images"] = []string{"https://user:pass@example.com/a.png"} }},
		{"long URL", func(p map[string]any) { p["images"] = []string{"https://example.com/" + strings.Repeat("a", 2048)} }},
		{"unknown condition", func(p map[string]any) { p["condition"] = "崭新" }},
		{"missing condition", func(p map[string]any) { delete(p, "condition") }},
		{"empty title", func(p map[string]any) { p["title"] = " " }},
		{"long title", func(p map[string]any) { p["title"] = strings.Repeat("字", 121) }},
		{"long description", func(p map[string]any) { p["description"] = strings.Repeat("字", 4001) }},
		{"zero price", func(p map[string]any) { p["priceCents"] = 0 }},
		{"negative price", func(p map[string]any) { p["priceCents"] = -1 }},
		{"fractional cents", func(p map[string]any) { p["priceCents"] = 12.5 }},
		{"price limit", func(p map[string]any) { p["priceCents"] = int64(1000000000001) }},
		{"forged seller", func(p map[string]any) { p["sellerId"] = "buyer" }},
		{"client expiry", func(p map[string]any) { p["expiresAt"] = "2100-01-01T00:00:00Z" }},
	} {
		t.Run(example.name, func(t *testing.T) {
			p := productBody("textbooks", map[string]any{"author": "A"})
			example.change(p)
			authRequest(t, base, "POST", "/products", token, bodyJSON(t, p), 400)
		})
	}
	for _, key := range []string{"priceCents", "images", "condition", "title", "description", "categoryId", "sellerId", "id", "initialPriceCents", "publishedAt", "expiresAt", "sold", "priceHistory"} {
		t.Run("reserved "+key, func(t *testing.T) {
			authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A", key: 1})), 400)
		})
	}
	for _, condition := range []string{"全新", "几乎全新", "轻度使用", "明显使用"} {
		p := productBody("textbooks", map[string]any{"author": strings.Repeat("字", 256), "mixed": []any{"a", 12.5, true}, "numbers": make([]int, 16), strings.Repeat("k", 40): false})
		p["condition"] = condition
		p["images"] = repeatedImages(9)
		p["images"].([]string)[0] = "HTTPS://example.invalid/image.png"
		p["priceCents"] = int64(1000000000000)
		p["title"] = strings.Repeat("字", 120)
		p["description"] = strings.Repeat("字", 4000)
		for i := 0; i < 13; i++ {
			p["attributes"].(map[string]any)[fmt.Sprintf("extra%d", i)] = true
		}
		published := authRequest(t, base, "POST", "/products", token, bodyJSON(t, p), 201)
		detail := request("GET", "/products/"+published["id"].(string), "", 200)["product"].(map[string]any)
		if detail["images"].([]any)[0] != "HTTPS://example.invalid/image.png" || len(detail["images"].([]any)) != 9 || detail["priceCents"] != float64(1000000000000) {
			t.Fatalf("boundary not preserved: %v", detail)
		}
	}
	p := productBody("textbooks", map[string]any{"author": "A"})
	authRequest(t, base, "POST", "/products", "", bodyJSON(t, p), 401)
	authRequest(t, base, "POST", "/products", token, bodyJSON(t, p)+" {}", 400)
	authRequest(t, base, "POST", "/products", token, strings.Repeat(" ", 65536)+bodyJSON(t, p), 400)
	authRequest(t, base, "POST", "/products", token, `{"priceCents":9223372036854775808}`, 400)
	request("GET", "/products/not-an-id", "", 400)
	request("GET", "/products/012345678901234567890123", "", 404)
}
func repeatedImages(n int) []string {
	images := make([]string, n)
	for i := range images {
		images[i] = "http://example.invalid/image.png"
	}
	return images
}

func TestRuleChangesPreserveReadsAndControlNewWritesAndOwnEdits(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	seller := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	buyer := request("POST", "/auth/login", `{"username":"buyer","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	input := productBody("textbooks", map[string]any{"author": "原作者"})
	published := authRequest(t, base, "POST", "/products", seller, bodyJSON(t, input), 201)
	id := published["id"].(string)
	// Platform maintenance is fixture setup, not an end-user category-write API.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := db.Collection("categories").UpdateOne(ctx, bson.M{"_id": "textbooks"}, bson.M{"$set": bson.M{"rules.syllabus": bson.M{"type": "boolean", "required": true}}})
	if err != nil {
		t.Fatal(err)
	}
	detail := request("GET", "/products/"+id, "", 200)
	if detail["product"].(map[string]any)["attributes"].(map[string]any)["author"] != "原作者" {
		t.Fatalf("historical read changed: %v", detail)
	}
	categories := request("GET", "/categories", "", 200)["items"].([]any)
	for _, value := range categories {
		row := value.(map[string]any)
		if row["id"] == "textbooks" && row["rules"].(map[string]any)["syllabus"] == nil {
			t.Fatal("current rules missing")
		}
	}
	authRequest(t, base, "POST", "/products", seller, bodyJSON(t, input), 400)
	delete(input, "priceCents")
	authRequest(t, base, "PUT", "/products/"+id, seller, bodyJSON(t, input), 400)
	input["attributes"] = map[string]any{"author": "新作者", "syllabus": true}
	input["title"] = "修改后的教材"
	authRequest(t, base, "PUT", "/products/"+id, "", bodyJSON(t, input), 401)
	authRequest(t, base, "PUT", "/products/"+id, buyer, bodyJSON(t, input), 403)
	input["sellerId"] = "buyer"
	authRequest(t, base, "PUT", "/products/"+id, seller, bodyJSON(t, input), 400)
	delete(input, "sellerId")
	input["priceCents"] = 999
	authRequest(t, base, "PUT", "/products/"+id, seller, bodyJSON(t, input), 400)
	delete(input, "priceCents")
	edited := authRequest(t, base, "PUT", "/products/"+id, seller, bodyJSON(t, input), 200)
	for _, key := range []string{"sellerId", "publishedAt", "expiresAt", "priceCents", "initialPriceCents"} {
		if edited[key] != published[key] {
			t.Fatalf("edit changed %s: %v", key, edited)
		}
	}
	if edited["title"] != "修改后的教材" {
		t.Fatalf("edit missing: %v", edited)
	}
	read := request("GET", "/products/"+id, "", 200)["product"].(map[string]any)
	if read["attributes"].(map[string]any)["syllabus"] != true {
		t.Fatalf("edit not persisted: %v", read)
	}
	// Current limits also govern writes; they are not applied to historical reads.
	_, err = db.Collection("categories").UpdateOne(ctx, bson.M{"_id": "textbooks"}, bson.M{"$set": bson.M{"limits.maxAttributes": 1}})
	if err != nil {
		t.Fatal(err)
	}
	authRequest(t, base, "PUT", "/products/"+id, seller, bodyJSON(t, input), 400)
	input["priceCents"] = 12345
	authRequest(t, base, "POST", "/products", seller, bodyJSON(t, input), 400)
	request("GET", "/products/"+id, "", 200)
}

func TestUnavailableProductsRemainReadableButCannotBeEdited(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, state := range []string{"expired", "delisted", "sold"} {
		t.Run(state, func(t *testing.T) {
			input := productBody("textbooks", map[string]any{"author": "A"})
			id := authRequest(t, base, "POST", "/products", token, bodyJSON(t, input), 201)["id"].(string)
			var err error
			switch state {
			case "expired":
				_, err = db.Collection("products").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"expiresAt": time.Now().Add(-time.Second)}})
			case "delisted":
				_, err = db.Collection("listings").DeleteOne(ctx, bson.M{"_id": id})
			case "sold":
				_, err = db.Collection("products").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"sold": true}})
			}
			if err != nil {
				t.Fatal(err)
			}
			delete(input, "priceCents")
			input["title"] = "不得修改"
			authRequest(t, base, "PUT", "/products/"+id, token, bodyJSON(t, input), 409)
			detail := request("GET", "/products/"+id, "", 200)
			if detail["available"] != false || detail["product"].(map[string]any)["title"] != "校园闲置" {
				t.Fatalf("unavailable archive: %v", detail)
			}
		})
	}
}

func TestPublishingRollsBackArchiveWhenEligibilityCannotBeCreated(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// A database validator reliably fails the second write, without a public fault endpoint.
	if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "listings"}, {Key: "validator", Value: bson.M{"blocked": bson.M{"$exists": true}}}, {Key: "validationLevel", Value: "strict"}}).Err(); err != nil {
		t.Fatal(err)
	}
	authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 503)
	for _, name := range []string{"products", "listings"} {
		n, err := db.Collection(name).CountDocuments(ctx, bson.M{})
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("partial publication: %s contains %d", name, n)
		}
	}
	if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "listings"}, {Key: "validator", Value: bson.M{}}}).Err(); err != nil {
		t.Fatal(err)
	}
	id := authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)["id"].(string)
	var listing struct {
		ExpiresAt time.Time `bson:"expiresAt"`
	}
	if err := db.Collection("listings").FindOne(ctx, bson.M{"_id": id}).Decode(&listing); err != nil {
		t.Fatal(err)
	}
	detail := request("GET", "/products/"+id, "", 200)
	expiry, err := time.Parse(time.RFC3339Nano, detail["product"].(map[string]any)["expiresAt"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if !listing.ExpiresAt.Equal(expiry) {
		t.Fatal("archive and eligibility deadlines differ")
	}
}

func TestDetailShowsRecentPriceChangesAndLinksToCompleteHistory(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	id := authRequest(t, base, "POST", "/products", token, bodyJSON(t, productBody("textbooks", map[string]any{"author": "A"})), 201)["id"].(string)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Fixture represents existing price changes; price mutation belongs to #7.
	changes := []any{}
	changedAt := time.Now().UTC().Truncate(time.Millisecond)
	for i := 1; i <= 7; i++ {
		changes = append(changes, bson.M{"_id": fmt.Sprintf("%024x", i), "productId": id, "oldPriceCents": 12345 - (i-1)*100, "newPriceCents": 12345 - i*100, "changedAt": changedAt})
	}
	if _, err := db.Collection("priceChanges").InsertMany(ctx, changes); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("products").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"priceCents": 11645}}); err != nil {
		t.Fatal(err)
	}
	// Another product's history must never appear in this product's response.
	if _, err := db.Collection("priceChanges").InsertOne(ctx, bson.M{"_id": fmt.Sprintf("%024x", 8), "productId": "other", "oldPriceCents": 99, "newPriceCents": 1, "changedAt": changedAt}); err != nil {
		t.Fatal(err)
	}
	detail := request("GET", "/products/"+id, "", 200)
	recent := detail["priceHistory"].([]any)
	if len(recent) != 5 || recent[0].(map[string]any)["newPriceCents"] != float64(11645) || recent[4].(map[string]any)["newPriceCents"] != float64(12045) {
		t.Fatalf("recent changes: %v", recent)
	}
	p := detail["product"].(map[string]any)
	if p["priceCents"] != float64(11645) || p["initialPriceCents"] != float64(12345) {
		t.Fatalf("initial price lost: %v", p)
	}
	historyURL := detail["priceHistoryUrl"].(string)
	cursor := ""
	seen := []string{}
	for page := 0; page < 4; page++ {
		path := historyURL + "?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		history := request("GET", path, "", 200)
		for _, value := range history["items"].([]any) {
			seen = append(seen, value.(map[string]any)["id"].(string))
		}
		cursor = history["nextCursor"].(string)
		if cursor == "" {
			break
		}
	}
	if len(seen) != 7 || cursor != "" {
		t.Fatalf("history omitted rows: %v cursor=%s", seen, cursor)
	}
	for i, value := range seen {
		if value != fmt.Sprintf("%024x", 7-i) {
			t.Fatalf("unstable history order: %v", seen)
		}
	}
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=abc", "?limit=", "?cursor=broken", "?cursor="} {
		request("GET", historyURL+query, "", 400)
	}
}

func TestProductSwaggerContract(t *testing.T) {
	_, request := setup(t, time.Hour)
	spec := request("GET", "/openapi.json", "", 200)
	paths := spec["paths"].(map[string]any)
	for path, methods := range map[string][]string{"/categories": {"get"}, "/products": {"post"}, "/products/{id}": {"get", "put"}, "/products/{id}/price-history": {"get"}} {
		raw, ok := paths[path]
		if !ok {
			t.Fatalf("undocumented path %s", path)
		}
		for _, method := range methods {
			operation, ok := raw.(map[string]any)[method]
			if !ok {
				t.Fatalf("undocumented operation %s %s", method, path)
			}
			if method == "post" || method == "put" {
				if len(operation.(map[string]any)["security"].([]any)) != 1 {
					t.Fatalf("undocumented ownership: %s", path)
				}
			}
		}
	}
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	publish := schemas["PublishProduct"].(map[string]any)
	props := publish["properties"].(map[string]any)
	images := props["images"].(map[string]any)
	pattern := images["items"].(map[string]any)["pattern"].(string)
	if matched, err := regexp.MatchString(pattern, "HTTPS://example.invalid/image.png"); err != nil || !matched {
		t.Fatal("image contract rejects an allowed HTTP scheme")
	}
	if images["minItems"] != float64(1) || images["maxItems"] != float64(9) || len(props["condition"].(map[string]any)["enum"].([]any)) != 4 || props["priceCents"].(map[string]any)["type"] != "integer" {
		t.Fatalf("product bounds missing: %v", props)
	}
}

func TestExpandedCategoryLimitsAndHistoricalReadsMatchContract(t *testing.T) {
	base, request, db := setupDatabase(t, time.Hour)
	token := request("POST", "/auth/login", `{"username":"seller","password":"CampusDemo123!"}`, 200)["accessToken"].(string)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rules := bson.M{"author": bson.M{"type": "string", "required": true}}
	attributes := map[string]any{"author": strings.Repeat("字", 257), "array": make([]int, 17), strings.Repeat("k", 41): true}
	for i := 0; i < 16; i++ {
		key := fmt.Sprintf("defined%d", i)
		rules[key] = bson.M{"type": "boolean", "required": false}
		attributes[key] = true
	}
	for i := 0; i < 17; i++ {
		attributes[fmt.Sprintf("extra%d", i)] = true
	}
	limits := bson.M{"maxAttributes": 40, "maxExtraAttributes": 20, "maxKeyLength": 60, "maxStringLength": 300, "maxArrayLength": 20}
	if _, err := db.Collection("categories").UpdateOne(ctx, bson.M{"_id": "textbooks"}, bson.M{"$set": bson.M{"rules": rules, "limits": limits}}); err != nil {
		t.Fatal(err)
	}
	input := productBody("textbooks", attributes)
	id := authRequest(t, base, "POST", "/products", token, bodyJSON(t, input), 201)["id"].(string)
	delete(input, "priceCents")
	authRequest(t, base, "PUT", "/products/"+id, token, bodyJSON(t, input), 200)
	// Shrinking rules must not make a previously valid archive unreadable.
	if _, err := db.Collection("categories").UpdateOne(ctx, bson.M{"_id": "textbooks"}, bson.M{"$set": bson.M{"limits.maxAttributes": 32, "limits.maxStringLength": 256, "limits.maxArrayLength": 16}}); err != nil {
		t.Fatal(err)
	}
	detail := request("GET", "/products/"+id, "", 200)["product"].(map[string]any)
	if len(detail["attributes"].(map[string]any)) != 36 {
		t.Fatalf("historical attributes lost: %v", detail)
	}
	authRequest(t, base, "PUT", "/products/"+id, token, bodyJSON(t, input), 400)
	spec := request("GET", "/openapi.json", "", 200)
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	for _, name := range []string{"PublishProduct", "ProductContent", "Product"} {
		attrs := schemas[name].(map[string]any)["properties"].(map[string]any)["attributes"].(map[string]any)
		if _, fixed := attrs["maxProperties"]; fixed {
			t.Fatalf("%s contract rejects valid historical or expanded attributes", name)
		}
	}
	values := schemas["AttributeValue"].(map[string]any)["oneOf"].([]any)
	for _, raw := range values {
		value := raw.(map[string]any)
		if _, fixed := value["maxLength"]; fixed {
			t.Fatal("contract fixes a mutable string limit")
		}
		if _, fixed := value["maxItems"]; fixed {
			t.Fatal("contract fixes a mutable array limit")
		}
	}
}
