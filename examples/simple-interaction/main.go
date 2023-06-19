package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

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

	colls, err := client.ListCollections(ctx)
	if err != nil {
		log.Fatalf("listing collections: %v", err)
	}
	log.Printf("found %d existing collections", len(colls))
	for _, c := range colls {
		log.Printf("  - existing collection: %+v", c)
	}

	// Create a new collection
	collName := fmt.Sprintf("coll-%d", time.Now().UnixMilli())
	meta := map[string]interface{}{"hnsw:space": "cosine"}
	embeddingFunc := &dummyEmbeddingGenerator{calls: make([][]string, 0)}

	coll, err := client.CreateCollection(
		ctx,
		collName,
		chroma.WithMetadata(meta),
		chroma.WithEmbeddingFunc(embeddingFunc),
	)
	if err != nil {
		log.Fatalf("creating collection: %v", err)
	}
	log.Printf("created collection: %+v", coll)

	// Update the collection
	updatedMeta := map[string]interface{}{"hnsw:space": "cosine", "second": "try"}
	updated, err := client.GetOrCreateCollection(
		ctx,
		collName,
		chroma.WithMetadata(updatedMeta),
		chroma.WithEmbeddingFunc(embeddingFunc),
	)
	if err != nil {
		log.Fatalf("updating collection: %v", err)
	}
	log.Printf("updated collection: %+v", updated)

	// Delete it
	if err := client.DeleteCollection(ctx, collName); err != nil {
		log.Fatalf("deleting collection: %v", err)
	}

	log.Printf("done!")
}

type dummyEmbeddingGenerator struct {
	calls [][]string
}

func (d *dummyEmbeddingGenerator) Generate(ctx context.Context, texts []string) ([]chroma.Embedding, error) {
	d.calls = append(d.calls, texts)
	return nil, nil
}
