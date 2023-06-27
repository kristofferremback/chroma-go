package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/kristofferostlund/chroma-go/chroma"
	"github.com/kristofferostlund/chroma-go/chroma/embeddings/openai"
)

var (
	chromaURL       = flag.String("chroma-url", "http://localhost:8000", "URL to chromadb server")
	openaiAuthToken = flag.String("openai-auth-token", "", "OpenAI API auth token")
)

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
	meta := map[string]interface{}{"hnsw:space": "cosine", "updated_at": time.Now().Format(time.RFC3339Nano)}
	embeddingFunc := openai.NewEmbeddingGenerator(*openaiAuthToken)

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
	collName += "-updated"
	updatedMeta := map[string]interface{}{"hnsw:space": "cosine", "updated_at": time.Now().Format(time.RFC3339Nano)}
	if err := coll.Modify(ctx, collName, updatedMeta); err != nil {
		log.Fatalf("updating collection: %v", err)
	}
	log.Printf("updated collection: %+v", coll)

	for i := 0; i < 3; i++ {
		success, err := coll.AddOne(ctx, fmt.Sprintf("id-%d", i+1), nil, chroma.Metadata{"this": "is fine", "index": fmt.Sprint(i)}, fmt.Sprintf("Hi %v", i))
		if err != nil {
			log.Fatalf("adding document: %v", err)
		}
		log.Printf("added 	document: %v", success)
	}

	success, err := coll.UpsertOne(ctx, "id-1", nil, chroma.Metadata{"this": "is fine", "index": "1", "updated": "true"}, "Hi 1")
	if err != nil {
		log.Fatalf("upserting document: %v", err)
	}
	log.Printf("upserted document: %v", success)

	count, err := coll.Count(ctx)
	if err != nil {
		log.Fatalf("counting documents: %v", err)
	}
	log.Printf("there are %d documents in the collection", count)

	// Delete it
	if err := client.DeleteCollection(ctx, collName); err != nil {
		log.Fatalf("deleting collection: %v", err)
	}

	log.Printf("done!")
}
