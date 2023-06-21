package openai

import (
	"context"
	"fmt"

	"github.com/kristofferostlund/chroma-go/chroma"
	"github.com/sashabaranov/go-openai"
)

var _ chroma.EmbeddingGenerator = (*EmbeddingGenerator)(nil)

type EmbeddingGenerator struct {
	openai *openai.Client
	model  openai.EmbeddingModel
}

type Config struct {
	authToken string
	orgID     string
	model     openai.EmbeddingModel
}

func (c *Config) OpenAIConfig() openai.ClientConfig {
	conf := openai.DefaultConfig(c.authToken)
	if c.orgID != "" {
		conf.OrgID = c.orgID
	}
	return conf
}

type Opt func(c *Config)

func Model(model openai.EmbeddingModel) Opt {
	return func(c *Config) {
		c.model = model
	}
}

func OrgID(orgID string) Opt {
	return func(c *Config) {
		c.orgID = orgID
	}
}

func NewEmbeddingGenerator(authToken string, opts ...Opt) *EmbeddingGenerator {
	conf := &Config{
		authToken: authToken,
		orgID:     "",
		model:     openai.AdaEmbeddingV2,
	}
	for _, opt := range opts {
		opt(conf)
	}

	return &EmbeddingGenerator{openai: openai.NewClientWithConfig(conf.OpenAIConfig()), model: conf.model}
}

func (e *EmbeddingGenerator) Generate(ctx context.Context, documents []chroma.Document) ([]chroma.Embedding, error) {
	resp, err := e.openai.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: documents,
		Model: e.model,
		User:  "",
	})
	if err != nil {
		return nil, fmt.Errorf("creating embeddings:%w", err)
	}

	embeddings := make([]chroma.Embedding, 0, len(resp.Data))
	for _, data := range resp.Data {
		embedding := make(chroma.Embedding, 0, len(data.Embedding))
		for _, v := range data.Embedding {
			embedding = append(embedding, float64(v))
		}

		embeddings = append(embeddings, embedding)
	}

	return embeddings, nil
}
