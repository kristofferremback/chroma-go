package chroma

import (
	"context"
	"errors"
	"fmt"

	"github.com/kristofferostlund/chroma-go/chroma/chromaclient"
	"github.com/kristofferostlund/chroma-go/pkg/nillable"
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

var _ EmbeddingGenerator = (*noEmbeddingGenerator)(nil)

type noEmbeddingGenerator struct{}

func (*noEmbeddingGenerator) Generate(ctx context.Context, documents []Document) ([]Embedding, error) {
	return nil, fmt.Errorf("%w: no embedding generator set", ErrInvalidInput)
}

func newCollection(id, name string, metadata Metadata, api chromaclient.ClientInterface, embeddingGen EmbeddingGenerator) *Collection {
	coll := &Collection{
		ID:       id,
		Name:     name,
		Metadata: metadata,

		api:          api,
		embeddingGen: &noEmbeddingGenerator{},
	}

	if embeddingGen != nil {
		coll.embeddingGen = embeddingGen
	}

	return coll
}

func (c *Collection) Add(ctx context.Context, ids []ID, embeddings []Embedding, metadatas []Metadata, documents []Document) (bool, error) {
	b, err := c.validatedSetEmbeddingRequest(ctx, ids, embeddings, metadatas, documents)
	if err != nil {
		return false, fmt.Errorf("validating: %w", err)
	}

	r, err := handleResponse(c.api.Add(ctx, c.ID, chromaclient.AddEmbedding(b)))
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
	b, err := c.validatedSetEmbeddingRequest(ctx, ids, embeddings, metadatas, documents)
	if err != nil {
		return false, fmt.Errorf("validating: %w", err)
	}

	r, err := handleResponse(c.api.Upsert(ctx, c.ID, chromaclient.AddEmbedding(b)))
	if err != nil {
		return false, fmt.Errorf("upserting: %w", err)
	}

	var success bool
	if err := r.decodeJSON(&success); err != nil {
		return false, fmt.Errorf("decoding response: %w", err)
	}

	return success, nil
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

func (c *Collection) Update(ctx context.Context, ids []ID, embeddings []Embedding, metadatas []Metadata, documents []Document) (bool, error) {
	b, err := c.validatedSetEmbeddingRequest(ctx, ids, embeddings, metadatas, documents)
	if err != nil {
		return false, fmt.Errorf("validating: %w", err)
	}

	r, err := handleResponse(c.api.Update(ctx, c.ID, chromaclient.UpdateEmbedding(b)))
	if err != nil {
		return false, fmt.Errorf("upserting: %w", err)
	}

	var success bool
	if err := r.decodeJSON(&success); err != nil {
		return false, fmt.Errorf("decoding response: %w", err)
	}

	return success, nil
}

func (c *Collection) UpdateOne(ctx context.Context, id ID, embedding Embedding, metadata Metadata, document Document) (bool, error) {
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

	return c.Update(ctx, ids, embeddings, metadatas, documents)
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

type Operator string

const (
	And      Operator = "$and"
	Or       Operator = "$or"
	Gt       Operator = "$gt"
	Gte      Operator = "$gte"
	Lt       Operator = "$lt"
	Lte      Operator = "$lte"
	Ne       Operator = "$ne"
	Eq       Operator = "$eq"
	Contains Operator = "$contains"
)

type (
	Where         = map[Operator]interface{}
	WhereDocument = map[Operator]interface{}

	GetQuery struct {
		IDs           []ID
		Where         Where
		Limit         int
		Offset        int
		Include       []Include
		WhereDocument WhereDocument
	}
)

func (c *Collection) Get(ctx context.Context, query GetQuery) (EmbeddingResponse, error) {
	body := chromaclient.GetEmbedding{}
	if len(query.IDs) > 0 {
		body.Ids = &query.IDs
	}
	if len(query.Where) > 0 {
		where := make(map[string]interface{}, len(query.Where))
		for k, v := range query.Where {
			where[string(k)] = v
		}
		body.Where = &where
	}
	if len(query.WhereDocument) > 0 {
		whereDoc := make(map[string]interface{}, len(query.WhereDocument))
		for k, v := range query.WhereDocument {
			whereDoc[string(k)] = v
		}
		body.WhereDocument = &whereDoc
	}
	if query.Limit > 0 {
		body.Limit = &query.Limit
	}
	if query.Offset > 0 {
		body.Offset = &query.Offset
	}
	if len(query.Include) > 0 {
		incl, err := validatedInclude(query.Include, false)
		if err != nil {
			return EmbeddingResponse{}, fmt.Errorf("validating include: %w", err)
		}
		body.Include = &incl
	}

	r, err := handleResponse(c.api.Get(ctx, c.ID, body))
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("querying collection (%s): %w", c.ID, err)
	}

	var response EmbeddingResponse
	if err := r.decodeJSON(&response); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("decoding response: %w", err)
	}

	return response, nil
}

type QueryQuery struct {
	Embeddings    []Embedding
	QueryTexts    []string
	Where         Where
	WhereDocument WhereDocument
	NResults      int
	Include       []Include
}

func (c *Collection) Query(ctx context.Context, query QueryQuery) (EmbeddingResponse, error) {
	body := chromaclient.QueryEmbedding{}
	switch {
	case len(query.Embeddings) > 0:
	case len(query.Embeddings) == 0 && len(query.QueryTexts) > 0:
		generatedEmbeddings, err := c.embeddingGen.Generate(ctx, query.QueryTexts)
		if err != nil {
			return EmbeddingResponse{}, fmt.Errorf("generating embeddings: %w", err)
		}
		body.
	case len(query.Embeddings) == 0 && len(query.QueryTexts) == 0:
		return EmbeddingResponse{}, fmt.Errorf("must provide at least one embedding or query text")

	}

	if query.NResults == 0 {
		body.NResults = nillable.Of(10)
	}

	return EmbeddingResponse{}, fmt.Errorf("not implemented")
}

type Include string

const (
	IncludeMetadatas  Include = "metadatas"
	IncludeDocuments  Include = "documents"
	IncludeEmbeddings Include = "embeddings"
	// IncludeDistances is not valid for Get.
	IncludeDistances Include = "distances"
)

var (
	IncludeAllGet   = includeAllExceptDistance
	IncludeAllQuery = includeAll

	includeAll               = []Include{IncludeMetadatas, IncludeDocuments, IncludeEmbeddings, IncludeDistances}
	includeAllExceptDistance = []Include{IncludeMetadatas, IncludeDocuments, IncludeEmbeddings}

	legalIncludes = map[Include]struct{}{
		IncludeMetadatas:  {},
		IncludeDocuments:  {},
		IncludeEmbeddings: {},
		IncludeDistances:  {},
	}
)

func validatedInclude(include []Include, allowDistances bool) ([]chromaclient.GetEmbeddingInclude, error) {
	incl := make([]chromaclient.GetEmbeddingInclude, 0, len(include))
	for _, v := range include {
		if _, ok := legalIncludes[v]; !ok {
			return nil, fmt.Errorf("%w: got include %q, must be one of %v", ErrInvalidInput, v, includeAllExceptDistance)
		}
		// When calling Get, there's no distance, however when calling Query, there is.
		// This validation is delegated to the client as it returns a 500 if included in
		// the request due to an internal database error: `Missing columns: 'distance'`.
		if v == IncludeDistances && !allowDistances {
			return nil, fmt.Errorf("%w: got include %q, must be one of %v", ErrInvalidInput, v, includeAllExceptDistance)
		}
		incl = append(incl, chromaclient.GetEmbeddingInclude(v))
	}
	return incl, nil
}

// Copied from types.gen.go to make it clear we're using this explicitly here.
// In case the types change, this shouldn't be castable to the generated types.
type setEmbedding struct {
	Documents      *[]string                 `json:"documents,omitempty"`
	Embeddings     *[][]float64              `json:"embeddings,omitempty"`
	Ids            []string                  `json:"ids"`
	IncrementIndex *bool                     `json:"increment_index,omitempty"`
	Metadatas      *[]map[string]interface{} `json:"metadatas,omitempty"`
}

func (c *Collection) validatedSetEmbeddingRequest(ctx context.Context, ids []ID, embeddings []Embedding, metadatas []Metadata, documents []Document) (setEmbedding, error) {
	if len(embeddings) == 0 && len(documents) == 0 {
		return setEmbedding{}, fmt.Errorf("%w: no embeddings or documents", ErrInvalidInput)
	}

	if len(embeddings) == 0 && len(documents) > 0 {
		generatedEmbeddings, err := c.embeddingGen.Generate(ctx, documents)
		if err != nil {
			return setEmbedding{}, fmt.Errorf("generating embeddings: %w", err)
		}
		// Feels a bitt iffy to generate embeddings here, but I don't want to
		// stray from the source too much just yet.
		embeddings = generatedEmbeddings
	}

	if len(embeddings) == 0 {
		return setEmbedding{}, fmt.Errorf("%w: no embeddings", ErrInvalidInput)
	}

	addEmbedding := setEmbedding{
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
