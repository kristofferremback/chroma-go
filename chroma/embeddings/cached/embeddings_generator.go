package cached

import (
	"context"
	"fmt"
	"sync"

	"github.com/kristofferostlund/chroma-go/chroma"
)

var _ chroma.EmbeddingGenerator = (*CachedEmbeddingsGenerator)(nil)

// CachedEmbeddingsGenerator wraps an embedding generator and caches
// results so that subsequent calls with the same document will return
// the same embeddings without having to generate them again.
type CachedEmbeddingsGenerator struct {
	generator chroma.EmbeddingGenerator

	lock    sync.Locker
	gen     chan genReq
	waiting map[chroma.Document][]chan res

	cache map[chroma.Document]chroma.Embedding
}

type genReq struct {
	ctx       context.Context
	documents []chroma.Document
}

type res struct {
	embedding chroma.Embedding
	err       error
}

func NewEmbeddingsGenerator(ctx context.Context, generator chroma.EmbeddingGenerator) *CachedEmbeddingsGenerator {
	gen := &CachedEmbeddingsGenerator{
		generator: generator,
		cache:     make(map[chroma.Document]chroma.Embedding),
		gen:       make(chan genReq),
		lock:      &sync.Mutex{},
		waiting:   make(map[chroma.Document][]chan res),
	}

	go gen.run(ctx)

	return gen
}

func (c *CachedEmbeddingsGenerator) Generate(ctx context.Context, documents []chroma.Document) ([]chroma.Embedding, error) {
	embeddingChans := c.requestEmbeddings(ctx, documents)

	embeddings := make([]chroma.Embedding, len(documents))
	for i := range embeddingChans {
		r := <-embeddingChans[i]
		if r.err != nil {
			return nil, fmt.Errorf("getting embedding: %w", r.err)
		}

		embeddings[i] = r.embedding
	}

	return embeddings, nil
}

func (c *CachedEmbeddingsGenerator) requestEmbeddings(ctx context.Context, docs []chroma.Document) []<-chan res {
	// We lock so we can safely update the cache.
	// Since we're using channels, the lock is active only when mutating the cache
	// or when adding channels to the waiting list.
	// All requests will be handled concurrently.
	c.lock.Lock()
	defer c.lock.Unlock()

	embeddingChans := make([]chan res, 0, len(docs))

	for i := 0; i < len(docs); i++ {
		// Populate with buffered channels so the generator can send
		// without having to wait for the receivers.
		embeddingChans = append(embeddingChans, make(chan res, 1))
	}

	docsToGenerate := make([]chroma.Document, 0)

	for i, doc := range docs {
		if embedding, ok := c.cache[doc]; ok {
			// It's in the cache, no need to generate.
			embeddingChans[i] <- res{embedding, nil}
			continue
		}

		if _, ok := c.waiting[doc]; !ok {
			// If we're not already waiting for this document, we need to generate it.
			// Since we hold the lock, we know no other goroutine is will want to generate
			// this document (unless this one fails).
			docsToGenerate = append(docsToGenerate, doc)
		}

		// Add ourselves to the waiting list regardless of whether we're generating or not.
		c.waiting[doc] = append(c.waiting[doc], embeddingChans[i])
	}

	if len(docsToGenerate) > 0 {
		// Dispatch a goroutine to generate the embeddings
		go func() { c.gen <- genReq{ctx, docsToGenerate} }()
	}

	// We can't cast a slice of channels to a slice of receive-only channels,
	// but we can append the two-way channel to a slice of receive-only channels.
	receiveChans := make([]<-chan res, 0, len(docs))
	for _, ch := range embeddingChans {
		receiveChans = append(receiveChans, ch)
	}
	return receiveChans
}

func (c *CachedEmbeddingsGenerator) handleGen(req genReq) {
	// We check the error further down so we can fail all waiting docs
	// in case of error.
	embeddings, err := c.generator.Generate(req.ctx, req.documents)
	// Error handling within nested loop below.

	// We lock so we can safely update the cache and waiting channels.
	c.lock.Lock()
	defer c.lock.Unlock()

	for i, doc := range req.documents {
		c.cache[doc] = embeddings[i]

		if _, ok := c.waiting[doc]; ok {
			for _, ch := range c.waiting[doc] {
				// We need to fail all waiting docs in case of error to ensure
				// all waiting goroutines receive the error.
				if err != nil {
					ch <- res{nil, fmt.Errorf("generating embedding for document: %w", err)}
					continue
				} else {
					ch <- res{embeddings[i], nil}
				}
			}

			// Clean up the waiting list.
			delete(c.waiting, doc)
		}
	}
}

func (c *CachedEmbeddingsGenerator) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-c.gen:
			go c.handleGen(req)
		}
	}
}
