package main

import (
	"context"
	"flag"
	"log"

	"github.com/kristofferostlund/chroma-go/chroma"
)

var chromaURL = flag.String("chroma-url", "http://localhost:8000", "URL to chromadb server")

func main() {
	flag.Parse()

	ctx := context.Background()
	client := chroma.NewClient(*chromaURL)

	version, err := client.Version(ctx)
	if err != nil {
		log.Fatalf("getting version: %v", err)
	}

	heartbeat, err := client.Heartbeat(ctx)
	if err != nil {
		log.Fatalf("sending heartbeat: %v", err)
	}

	log.Printf("chromadb version: %s", version)
	log.Printf("last heartbeat at %s", heartbeat)
}
