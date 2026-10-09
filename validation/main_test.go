package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
)

func TestChineseSynonyms(t *testing.T) {
	base := os.Getenv("API_URL")
	if base == "" {
		t.Fatal("API_URL required: use docker compose run --rm verify")
	}
	expected := 27
	if os.Getenv("SCALE") == "1" {
		expected = 20000
	}
	for _, q := range []string{"自行车", "单车", "脚踏车"} {
		resp, err := http.Get(base + "/search?q=" + q)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Total   int `json:"total"`
			Results []struct {
				ID            string   `json:"id"`
				MatchedFields []string `json:"matchedFields"`
				Highlights    []any    `json:"highlights"`
			} `json:"results"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("search: %d %v", resp.StatusCode, err)
		}
		if result.Total != expected || len(result.Results) != 20 {
			t.Fatalf("%s: total=%d results=%d want %d/20", q, result.Total, len(result.Results), expected)
		}
		fields := map[string]bool{}
		both := false
		seen := map[string]bool{}
		for _, r := range result.Results {
			var n int
			if _, err := fmt.Sscanf(r.ID, "p-%06d", &n); err != nil || n < 0 || n >= expected || seen[r.ID] {
				t.Fatalf("invalid or duplicate result %s", r.ID)
			}
			seen[r.ID] = true
			if len(r.Highlights) == 0 {
				t.Fatal("missing snippets")
			}
			for _, f := range r.MatchedFields {
				fields[f] = true
			}
			if len(r.MatchedFields) == 2 {
				both = true
			}
		}
		if os.Getenv("SCALE") != "1" && (!fields["title"] || !fields["description"] || !both) {
			t.Fatalf("missing field coverage: %v both=%v", fields, both)
		}
	}
}
