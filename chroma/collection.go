package chroma

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/kristofferostlund/chroma-go/chroma/chromaclient"
)

var ErrInvalidInput = errors.New("invalid input")

type EmbeddingResponse struct {
	IDs        []string    `json:"ids"`
	Embeddings []Embedding `json:"embeddings"`
	Documents  []Document  `json:"documents"`
	Metadatas  []Metadata  `json:"metadatas"`
}

type Collection struct {
	ID       string
	Name     string
	Metadata Metadata

	api          chromaclient.ClientInterface
	embeddingGen EmbeddingGenerator
}

func (c *Collection) Add(ctx context.Context, ids []ID, embeddings []Embedding, metadatas []Metadata, documents []Document) (bool, error) {
	body, err := c.validatedAddEmbedding(ctx, ids, embeddings, metadatas, documents)
	if err != nil {
		return false, fmt.Errorf("validating: %w", err)
	}

	r, err := handleResponse(c.api.Add(ctx, c.ID, body))
	if err != nil {
		return false, fmt.Errorf("adding: %w", err)
	}

	var success bool
	if err := r.decodeJSON(&success); err != nil {
		return false, fmt.Errorf("decoding response: %w", err)
	}

	return success, nil
}

func (c *Collection) AddOne(ctx context.Context, id ID, embedding Embedding, metadata Metadata, document Document) (bool, error) {
	ids := []string{id}
	embeddings := []Embedding{}
	if len(embedding) > 0 {
		embeddings = []Embedding{embedding}
	}

	metadatas := []Metadata{}
	if len(metadata) > 0 {
		metadatas = []Metadata{metadata}
	}

	documents := []Document{}
	if len(document) > 0 {
		documents = []Document{document}
	}

	return c.Add(ctx, ids, embeddings, metadatas, documents)
}

func (c *Collection) Upsert(ctx context.Context, ids []ID, embeddings []Embedding, metadatas []Metadata, documents []Document) (bool, error) {
	body, err := c.validatedAddEmbedding(ctx, ids, embeddings, metadatas, documents)
	if err != nil {
		return false, fmt.Errorf("validating: %w", err)
	}

	r, err := handleResponse(c.api.Upsert(ctx, c.ID, body))
	if err != nil {
		return false, fmt.Errorf("upserting: %w", err)
	}

	var success bool
	if err := r.decodeJSON(&success); err != nil {
		return false, fmt.Errorf("decoding response: %w", err)
	}

	return success, nil
}

func (c *Collection) Count(ctx context.Context) (int, error) {
	r, err := handleResponse(c.api.Count(ctx, c.ID))
	if err != nil {
		return 0, fmt.Errorf("counting: %w", err)
	}

	var count int
	if err := r.decodeJSON(&count); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	return count, nil
}

func (c *Collection) UpsertOne(ctx context.Context, id ID, embedding Embedding, metadata Metadata, document Document) (bool, error) {
	ids := []string{id}
	embeddings := []Embedding{}
	if len(embedding) > 0 {
		embeddings = []Embedding{embedding}
	}

	metadatas := []Metadata{}
	if len(metadata) > 0 {
		metadatas = []Metadata{metadata}
	}

	documents := []Document{}
	if len(document) > 0 {
		documents = []Document{document}
	}

	return c.Upsert(ctx, ids, embeddings, metadatas, documents)
}

func (c *Collection) Modify(ctx context.Context, name string, metadata Metadata) error {
	body := chromaclient.UpdateCollection{
		NewMetadata: nil,
		NewName:     nil,
	}

	if name != "" {
		body.NewName = &name
	}
	if len(metadata) > 0 {
		body.NewMetadata = &metadata
	}

	if _, err := handleResponse(c.api.UpdateCollection(ctx, c.ID, body)); err != nil {
		return fmt.Errorf("modifying: %w", err)
	}

	if name != "" {
		c.Name = name
	}
	if len(metadata) > 0 {
		c.Metadata = metadata
	}

	return nil
}

func (c *Collection) validatedAddEmbedding(ctx context.Context, ids []ID, embeddings []Embedding, metadatas []Metadata, documents []Document) (chromaclient.AddEmbedding, error) {
	if len(embeddings) == 0 && len(documents) == 0 {
		return chromaclient.AddEmbedding{}, fmt.Errorf("%w: no embeddings or documents", ErrInvalidInput)
	}

	if len(embeddings) == 0 && len(documents) > 0 {
		if c.embeddingGen == nil {
			return chromaclient.AddEmbedding{}, fmt.Errorf("%w: no embedding generator", ErrInvalidInput)
		}

		log.Printf("[collection] must generate embeddings for docs: %v", documents)

		generatedEmbeddings, err := c.embeddingGen.Generate(ctx, documents)
		if err != nil {
			return chromaclient.AddEmbedding{}, fmt.Errorf("generating embeddings: %w", err)
		}
		// Feels a bitt iffy to generate embeddings here, but I don't want to
		// stray from the source too much just yet.
		embeddings = generatedEmbeddings
	}

	if len(embeddings) == 0 {
		return chromaclient.AddEmbedding{}, fmt.Errorf("%w: no embeddings", ErrInvalidInput)
	}

	addEmbedding := chromaclient.AddEmbedding{
		Ids:            ids,         // ids are explicitly required in the API
		Embeddings:     &embeddings, // embeddings are implicitly required in the API
		Documents:      nil,         // optional
		IncrementIndex: nil,         // optional
		Metadatas:      nil,         // optional
	}

	if len(documents) > 0 {
		addEmbedding.Documents = &documents
	}
	if len(metadatas) > 0 {
		addEmbedding.Metadatas = &metadatas
	}

	return addEmbedding, nil
}
