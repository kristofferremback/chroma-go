package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/kristofferostlund/chroma-go/chroma"
	"github.com/kristofferostlund/chroma-go/chroma/embeddings/cached"
	"github.com/kristofferostlund/chroma-go/chroma/embeddings/openai"
	"golang.org/x/sync/errgroup"
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
		if err := client.DeleteCollection(ctx, c.Name); err != nil {
			log.Fatalf("deleting collection: %v", err)
		}
	}

	// Create a new collection
	collName := fmt.Sprintf("coll-%d", time.Now().UnixMilli())
	meta := map[string]interface{}{"hnsw:space": "cosine", "updated_at": time.Now().Format(time.RFC3339Nano)}
	// embeddingFunc := openai.NewEmbeddingGenerator(*openaiAuthToken)
	embeddingFunc := cached.NewEmbeddingsGenerator(ctx, openai.NewEmbeddingGenerator(*openaiAuthToken))

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

	useBatch := true
	docCount := 500

	if useBatch {
		ids := make([]chroma.ID, 0, docCount/2)
		metadatas := make([]chroma.Metadata, 0, docCount)
		docs := make([]chroma.Document, 0, docCount)
		for i := 0; i < docCount; i++ {
			id, metadata, doc := buildEmbeddingDoc(i, "add-batch")
			ids = append(ids, id)
			metadatas = append(metadatas, metadata)
			docs = append(docs, doc)
		}

		success, err := coll.Add(ctx, ids, nil, metadatas, docs)
		if err != nil {
			log.Fatalf("adding documents %d: %v", len(ids), err)
		}
		log.Printf("added %d documents: %v", len(docs), success)
	} else {
		eg, ctx := errgroup.WithContext(ctx)
		for i := 0; i < docCount; i++ {
			idx := i
			eg.Go(func() error {
				log.Printf("running %d", idx)

				id, metadata, doc := buildEmbeddingDoc(idx, "add")
				success, err := coll.AddOne(ctx, id, nil, metadata, doc)
				if err != nil {
					return fmt.Errorf("adding document (%d): %w", idx, err)
				}
				log.Printf("added document: %v", success)
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			log.Fatalf("adding documents: %v", err)
		}
	}

	// Separate blocks for scoping id, metadata, doc
	{
		id, metadata, doc := buildEmbeddingDoc(0, "upsert")
		upsertSuccess, err := coll.UpsertOne(ctx, id, nil, metadata, doc)
		if err != nil {
			log.Fatalf("upserting document: %v", err)
		}
		log.Printf("upserted document: %v", upsertSuccess)
	}

	// Separate blocks for scoping id, metadata, doc
	{
		id, metadata, doc := buildEmbeddingDoc(1, "update")
		doc += " updated"
		updateSuccess, err := coll.UpdateOne(ctx, id, nil, metadata, doc)
		if err != nil {
			log.Fatalf("updating document: %v", err)
		}
		log.Printf("updated document: %v", updateSuccess)
	}

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

func buildEmbeddingDoc(idx int, operation string) (chroma.ID, chroma.Metadata, chroma.Document) {
	doc := "Hi there"
	if idx%2 == 0 {
		doc = fmt.Sprintf("Hi %d", idx)
	}

	id := fmt.Sprintf("id-%d", idx+1)
	metadata := chroma.Metadata{"operation": operation, "index": fmt.Sprint(idx), "updated_at": time.Now().Format(time.RFC3339Nano)}
	return id, metadata, doc
}
