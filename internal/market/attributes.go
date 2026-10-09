package market

import (
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var attributeKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

func validText(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && length >= min && length <= max && (min == 0 || strings.TrimSpace(s) != "")
}
func validImage(s string) bool {
	u, err := url.Parse(s)
	return err == nil && len(s) <= 2048 && u.Hostname() != "" && (u.Scheme == "http" || u.Scheme == "https") && u.User == nil
}
func reservedAttribute(key string) bool {
	switch strings.ToLower(key) {
	case "id", "categoryid", "title", "description", "condition", "images", "attributes", "sellerid", "pricecents", "initialpricecents", "publishedat", "expiresat", "sold", "seller", "available", "pricehistory", "pricehistoryurl", "buyerid", "salepricecents", "confirmedat":
		return true
	}
	return false
}
func scalarType(value any, maxLength int) string {
	switch v := value.(type) {
	case string:
		if validText(v, 0, maxLength) {
			return "string"
		}
	case float64:
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return "number"
		}
	case bool:
		return "boolean"
	}
	return ""
}
func validAttribute(value any, ruleType string, limits AttributeLimits) bool {
	if values, ok := value.([]any); ok {
		if len(values) > limits.MaxArrayLength || (ruleType != "" && !strings.HasSuffix(ruleType, "[]")) {
			return false
		}
		for _, item := range values {
			kind := scalarType(item, limits.MaxStringLength)
			if kind == "" || (ruleType != "" && kind != strings.TrimSuffix(ruleType, "[]")) {
				return false
			}
		}
		return true
	}
	kind := scalarType(value, limits.MaxStringLength)
	return kind != "" && (ruleType == "" || ruleType == kind)
}
func validateContent(c *gin.Context, db *mongo.Database, p ProductContent) bool {
	if !validText(p.Title, 1, 120) || !validText(p.Description, 0, 4000) || len(p.Images) < 1 || len(p.Images) > 9 || p.Attributes == nil {
		fail(c, 400, "invalid_input")
		return false
	}
	switch p.Condition {
	case "全新", "几乎全新", "轻度使用", "明显使用":
	default:
		fail(c, 400, "invalid_input")
		return false
	}
	for _, image := range p.Images {
		if !validImage(image) {
			fail(c, 400, "invalid_input")
			return false
		}
	}
	var category Category
	err := db.Collection("categories").FindOne(c.Request.Context(), bson.M{"_id": p.CategoryID}).Decode(&category)
	if err == mongo.ErrNoDocuments {
		fail(c, 400, "invalid_input")
		return false
	}
	if err != nil {
		fail(c, 503, "unavailable")
		return false
	}
	if len(p.Attributes) > category.Limits.MaxAttributes {
		fail(c, 400, "invalid_input")
		return false
	}
	for key, rule := range category.Rules {
		if _, exists := p.Attributes[key]; rule.Required && !exists {
			fail(c, 400, "invalid_input")
			return false
		}
	}
	extras := 0
	for key, value := range p.Attributes {
		rule, defined := category.Rules[key]
		if !defined {
			extras++
		}
		if !attributeKey.MatchString(key) || len(key) > category.Limits.MaxKeyLength || reservedAttribute(key) || !validAttribute(value, rule.Type, category.Limits) {
			fail(c, 400, "invalid_input")
			return false
		}
	}
	if extras > category.Limits.MaxExtraAttributes {
		fail(c, 400, "invalid_input")
		return false
	}
	return true
}
