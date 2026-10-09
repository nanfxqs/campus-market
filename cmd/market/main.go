package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nanfxqs/campus-market/internal/market"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "serve" && os.Args[1] != "seed") {
		log.Fatal("usage: market serve|seed")
	}
	uri := os.Getenv("MONGO_URI")
	name := os.Getenv("DB_NAME")
	if uri == "" || name == "" || name == "admin" || name == "local" || name == "config" {
		log.Fatal("MONGO_URI and a non-system DB_NAME are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		if err := market.Seed(ctx, db); err != nil {
			log.Fatal(err)
		}
		log.Print("demo accounts: seller, buyer; password: CampusDemo123!")
		return
	}
	if err := market.Initialize(ctx, db); err != nil {
		log.Fatal(err)
	}
	server := http.Server{Addr: ":8080", Handler: market.New(db, time.Hour), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	log.Fatal(server.ListenAndServe())
}
