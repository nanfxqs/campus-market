package market

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RedisConfig addresses a dedicated, ephemeral experiment Redis database.
type RedisConfig struct {
	Address      string
	TTL, Timeout time.Duration
}
type detailCache struct {
	config                          RedisConfig
	hits, misses, errors, fallbacks atomic.Uint64
	requests, queries, bytes        atomic.Uint64
	lastLog                         atomic.Int64
}
type cachedContent struct {
	CategoryID  string         `json:"categoryId"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Images      []string       `json:"images"`
	Attributes  map[string]any `json:"attributes"`
}

func cacheContent(p Product) cachedContent {
	return cachedContent{p.CategoryID, p.Title, p.Description, p.Images, p.Attributes}
}
func (v cachedContent) apply(p *Product) {
	p.CategoryID = v.CategoryID
	p.Title = v.Title
	p.Description = v.Description
	p.Images = v.Images
	p.Attributes = v.Attributes
}

// command uses one connection per operation and a deadline bounding connect/read/write.
func (r RedisConfig) command(ctx context.Context, args ...string) ([]byte, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 100 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", r.Address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	var out strings.Builder
	fmt.Fprintf(&out, "*%d\r\n", len(args))
	for _, s := range args {
		fmt.Fprintf(&out, "$%d\r\n%s\r\n", len(s), s)
	}
	if _, err = io.WriteString(conn, out.String()); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 3 {
		return nil, fmt.Errorf("invalid Redis reply")
	}
	switch line[0] {
	case '+':
		return []byte(strings.TrimSuffix(line[1:], "\r\n")), nil
	case '-':
		return nil, fmt.Errorf("Redis: %s", line[1:])
	case '$':
		n, e := strconv.Atoi(strings.TrimSpace(line[1:]))
		if e != nil {
			return nil, e
		}
		if n == -1 {
			return nil, nil
		}
		if n < 0 || n > 1<<20 {
			return nil, fmt.Errorf("invalid Redis bulk size")
		}
		b := make([]byte, n+2)
		_, e = io.ReadFull(reader, b)
		return b[:n], e
	}
	return nil, fmt.Errorf("unexpected Redis reply")
}

// ClearDetailCache must only be used on the dedicated experiment Redis instance.
func ClearDetailCache(ctx context.Context, r RedisConfig) error {
	_, err := r.command(ctx, "FLUSHDB")
	return err
}
func NewWithCache(db *mongo.Database, ttl time.Duration, config RedisConfig) http.Handler {
	return newAPIWithCache(db, ttl, time.Now, &detailCache{config: config})
}

func (cache *detailCache) begin(c *gin.Context) func() {
	metrics := &detailReadMetrics{}
	c.Set("detailReadMetrics", metrics)
	return func() {
		cache.requests.Add(1)
		cache.queries.Add(uint64(metrics.queries))
		cache.bytes.Add(uint64(metrics.bytes))
		now := time.Now().Unix()
		last := cache.lastLog.Load()
		if last < now && cache.lastLog.CompareAndSwap(last, now) {
			log.Printf("detail_metrics scope=detail_returned_bson requests=%d hits=%d misses=%d errors=%d fallbacks=%d mongo_queries=%d mongo_bson_bytes=%d", cache.requests.Load(), cache.hits.Load(), cache.misses.Load(), cache.errors.Load(), cache.fallbacks.Load(), cache.queries.Load(), cache.bytes.Load())
		}
	}
}

func (cache *detailCache) load(c *gin.Context, db *mongo.Database, p *Product) bool {
	metrics := detailMetrics(c)
	c.Header("X-Detail-Cache", "disabled")
	if cache.config.Address == "" {
		if !validProductID(c) {
			return false
		}
		metrics.queries++
		raw, err := db.Collection("products").FindOne(c.Request.Context(), bson.M{"_id": c.Param("id")}).Raw()
		metrics.bytes += len(raw)
		if err == nil {
			err = bson.Unmarshal(raw, p)
		}
		return productLoadError(c, err)
	}
	// Dynamic archive fields are always read from MongoDB. Only the five explicit
	// stable fields are excluded; condition and any future fields stay dynamic.
	projection := bson.M{"title": 0, "description": 0, "images": 0, "categoryId": 0, "attributes": 0}
	read := func(filter bson.M, projection bson.M, out *Product) error {
		metrics.queries++
		raw, err := db.Collection("products").FindOne(c.Request.Context(), filter, options.FindOne().SetProjection(projection)).Raw()
		metrics.bytes += len(raw)
		if err != nil {
			return err
		}
		return bson.Unmarshal(raw, out)
	}
	if !validProductID(c) {
		return false
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := read(bson.M{"_id": c.Param("id")}, projection, p); err != nil {
			return productLoadError(c, err)
		}
		c.Header("X-Detail-Cache", "miss")
		key := fmt.Sprintf("detail:%s:%d", p.ID, p.ContentVersion)
		data, err := cache.config.command(c.Request.Context(), "GET", key)
		if err == nil && data != nil {
			var content cachedContent
			if json.Unmarshal(data, &content) == nil {
				content.apply(p)
				c.Header("X-Detail-Cache", "hit")
				cache.hits.Add(1)
				return true
			}
			err = fmt.Errorf("invalid cache JSON")
		}
		if err != nil {
			cache.errors.Add(1)
			metrics.cacheErrors++
			break
		}
		cache.misses.Add(1)
		filter := bson.M{"_id": p.ID, "contentVersion": p.ContentVersion}
		if p.ContentVersion == 0 {
			filter = bson.M{"_id": p.ID, "$or": bson.A{bson.M{"contentVersion": 0}, bson.M{"contentVersion": bson.M{"$exists": false}}}}
		}
		var full Product
		err = read(filter, nil, &full)
		if err == mongo.ErrNoDocuments {
			continue
		}
		if err != nil {
			return productLoadError(c, err)
		}
		content := cacheContent(full)
		content.apply(p)
		encoded, _ := json.Marshal(content)
		ttl := cache.config.TTL
		if ttl <= 0 {
			ttl = 10 * time.Minute
		}
		if _, err := cache.config.command(c.Request.Context(), "SET", key, string(encoded), "PX", strconv.FormatInt(max(1, ttl.Milliseconds()), 10)); err != nil {
			cache.errors.Add(1)
			metrics.cacheErrors++
		}
		return true
	}
	c.Header("X-Detail-Cache", "fallback")
	cache.fallbacks.Add(1)
	return productLoadError(c, read(bson.M{"_id": c.Param("id")}, nil, p))
}

type detailReadMetrics struct{ queries, bytes, cacheErrors int }

func detailMetrics(c *gin.Context) *detailReadMetrics {
	value, _ := c.Get("detailReadMetrics")
	return value.(*detailReadMetrics)
}
