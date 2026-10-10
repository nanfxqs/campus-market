package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nanfxqs/campus-market/internal/market"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "serve" && os.Args[1] != "seed") {
		log.Fatal("usage: market serve | seed [--mode full|small|stress|demo] [--random-seed 42] [--base-time RFC3339] [--reset]")
	}
	seed := market.SeedOptions{}
	timeout := 30 * time.Second
	if os.Args[1] == "seed" {
		flags := flag.NewFlagSet("seed", flag.ExitOnError)
		flags.StringVar(&seed.Mode, "mode", "full", "dataset: full, small, stress (30000 sellable products), demo (accounts only)")
		flags.Int64Var(&seed.RandomSeed, "random-seed", 42, "deterministic PRNG seed")
		base := flags.String("base-time", "2026-10-10T00:00:00Z", "fixed reference time (RFC3339); eligibility and TTL still use wall clock")
		flags.BoolVar(&seed.Reset, "reset", false, "DROP ONLY the database explicitly named by DB_NAME before importing")
		flags.DurationVar(&timeout, "timeout", 15*time.Minute, "maximum import duration")
		flags.Parse(os.Args[2:])
		if flags.NArg() != 0 || timeout <= 0 {
			log.Fatal("unexpected positional arguments or nonpositive timeout")
		}
		var err error
		seed.BaseTime, err = time.Parse(time.RFC3339, *base)
		if err != nil {
			log.Fatal("--base-time must be RFC3339: ", err)
		}
	} else if len(os.Args) != 2 {
		log.Fatal("serve does not accept arguments")
	}
	uri := os.Getenv("MONGO_URI")
	name := os.Getenv("DB_NAME")
	if uri == "" || name == "" || name == "admin" || name == "local" || name == "config" {
		log.Fatal("MONGO_URI and a non-system DB_NAME are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}
	db := client.Database(name)
	if os.Args[1] == "seed" {
		counts, err := market.SeedDataset(ctx, db, seed)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(map[string]any{"database": name, "mode": seed.Mode, "randomSeed": seed.RandomSeed, "baseTime": seed.BaseTime, "counts": counts})
		log.Print("demo accounts: seller, buyer; password: CampusDemo123!")
		return
	}
	if err := market.Initialize(ctx, db); err != nil {
		log.Fatal(err)
	}
	server := http.Server{Addr: ":8080", Handler: market.New(db, time.Hour), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	log.Fatal(server.ListenAndServe())
}
