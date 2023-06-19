package chroma

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"time"

	"github.com/kristofferostlund/chroma-go/chroma/chromaclient"
)

type Collection struct {
	ID       string
	Name     string
	Metadata map[string]interface{}

	api          chromaclient.ClientInterface
	embeddingGen EmbeddingGenerator
}

type SimpleCollection struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

type Embedding = []float64

type EmbeddingGenerator interface {
	Generate(ctx context.Context, texts []string) ([]Embedding, error)
}

type Client struct {
	api chromaclient.ClientInterface
}

func NewClient(path string) *Client {
	if path == "" {
		path = "http://localhost:8000"
	}
	api, err := chromaclient.NewClient(path)
	if err != nil {
		panic(fmt.Errorf("creating client: %w", err))
	}

	return &Client{api}
}

func (c *Client) Reset(ctx context.Context) error {
	if _, err := handleResponse(c.api.Reset(ctx)); err != nil {
		return fmt.Errorf("resetting: %w", err)
	}

	return nil
}

func (c *Client) Version(ctx context.Context) (string, error) {
	h, err := handleResponse(c.api.Version(ctx))
	if err != nil {
		return "", fmt.Errorf("getting version: %w", err)
	}

	var version string
	if err := h.decodeJSON(&version); err != nil {
		return "", fmt.Errorf("getting version: %w", err)
	}

	return version, nil
}

func (c *Client) Heartbeat(ctx context.Context) (time.Time, error) {
	h, err := handleResponse(c.api.Heartbeat(ctx))
	if err != nil {
		return time.Time{}, fmt.Errorf("sending heartbeat: %w", err)
	}

	var res struct {
		NanosecondHeartbeat *big.Int `json:"nanosecond heartbeat"`
	}
	if err := h.decodeJSON(&res); err != nil {
		return time.Time{}, fmt.Errorf("sending heartbeat: %w", err)
	}

	var ns int64
	if !res.NanosecondHeartbeat.IsInt64() {
		// At time of writing, the server returns the number of nanoseconds since epoch
		// **multiplied by 1000** which is also why we're using big.Int.
		// We could probably perform a better check, but this does the trick.
		// See below:
		// ```python
		// def heartbeat(self) -> int:
		//     """Ping the database to ensure it is alive
		//     Returns:
		//         The current time in milliseconds
		//     """
		//     return int(1000 * time.time_ns())` # <-- this right here
		// ```
		// Reported the issue here: https://github.com/chroma-core/chroma/issues/711
		// and made a PR here: https://github.com/chroma-core/chroma/pull/712
		ns = res.NanosecondHeartbeat.Div(res.NanosecondHeartbeat, big.NewInt(1000)).Int64()
	} else {
		ns = res.NanosecondHeartbeat.Int64()
	}

	at := time.Unix(0, ns)
	if math.Abs(float64(time.Now().Year()-at.Year())) > 10 {
		// If the year is off by more than 10 years, the bug has probably been fixed
		// and we're now getting the time in milliseconds as per the in-code comment.
		// I feel like I'm wearing such a tinfoil hat right now.
		at = time.UnixMilli(ns)
	}

	return at, nil
}

type collectionOpts struct {
	createOrGet   bool
	metadata      map[string]interface{}
	embeddingFunc EmbeddingGenerator
}

type CollectionOpts func(*collectionOpts)

func WithMetadata(metadata map[string]interface{}) CollectionOpts {
	return func(c *collectionOpts) {
		c.metadata = metadata
	}
}

func WithEmbeddingFunc(embeddingFunc EmbeddingGenerator) CollectionOpts {
	return func(c *collectionOpts) {
		c.embeddingFunc = embeddingFunc
	}
}

func (c *Client) CreateCollection(ctx context.Context, name string, opts ...CollectionOpts) (*Collection, error) {
	collOpts := collOptsOf(opts)
	// This is the explicit create function, we want to fail if the collection already exists.
	collOpts.createOrGet = false

	return c.createOrGetCollection(ctx, name, collOpts)
}

func (c *Client) GetOrCreateCollection(ctx context.Context, name string, opts ...CollectionOpts) (*Collection, error) {
	collOpts := collOptsOf(opts)
	collOpts.createOrGet = true

	return c.createOrGetCollection(ctx, name, collOpts)
}

func (c *Client) GetCollection(ctx context.Context, name string, opts ...CollectionOpts) (*Collection, error) {
	collOpts := collOptsOf(opts)
	if len(collOpts.metadata) > 0 {
		return nil, fmt.Errorf("cannot set metadata when getting collection, use GetOrCreateCollection to update the metadata")
	}

	r, err := handleResponse(c.api.GetCollection(ctx, name))
	if err != nil {
		return nil, fmt.Errorf("getting collection: %w", err)
	}

	var simpleColl SimpleCollection
	if err := r.decodeJSON(&simpleColl); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return c.collectionOf(simpleColl, collOpts), nil
}

func (c *Client) ListCollections(ctx context.Context) ([]SimpleCollection, error) {
	r, err := handleResponse(c.api.ListCollections(ctx))
	if err != nil {
		return nil, fmt.Errorf("listing collections: %w", err)
	}

	var collections []SimpleCollection
	if err := r.decodeJSON(&collections); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return collections, nil
}

func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	if _, err := handleResponse(c.api.DeleteCollection(ctx, name)); err != nil {
		return fmt.Errorf("deleting collection: %w", err)
	}
	return nil
}

func (c *Client) createOrGetCollection(ctx context.Context, name string, collOpts *collectionOpts) (*Collection, error) {
	body := chromaclient.CreateCollection{
		Name:        name,
		Metadata:    &collOpts.metadata,
		GetOrCreate: &collOpts.createOrGet,
	}

	r, err := handleResponse(c.api.CreateCollection(ctx, body))
	if err != nil {
		return nil, fmt.Errorf("creating collection: %w", err)
	}

	var simpleColl SimpleCollection
	if err := r.decodeJSON(&simpleColl); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return c.collectionOf(simpleColl, collOpts), nil
}

func (c *Client) collectionOf(simpleColl SimpleCollection, collOpts *collectionOpts) *Collection {
	coll := &Collection{
		ID:       simpleColl.ID,
		Name:     simpleColl.Name,
		Metadata: simpleColl.Metadata,

		api:          c.api,
		embeddingGen: nil,
	}

	// embeddingGen is optional.
	if collOpts.embeddingFunc != nil {
		coll.embeddingGen = collOpts.embeddingFunc
	}
	return coll
}

func collOptsOf(opts []CollectionOpts) *collectionOpts {
	collOpts := &collectionOpts{}
	for _, opt := range opts {
		opt(collOpts)
	}
	return collOpts
}

type requestWrapper struct {
	res *http.Response
}

func handleResponse(res *http.Response, err error) (*requestWrapper, error) {
	if err != nil {
		return nil, fmt.Errorf("requesting: %w", err)
	}

	if got, want := res.StatusCode, http.StatusOK; got != want {
		return nil, fmt.Errorf("requesting: got status %d, want %d", got, want)
	}

	return &requestWrapper{res}, nil
}

func (h *requestWrapper) decodeJSON(out any) error {
	defer h.res.Body.Close()
	if err := json.NewDecoder(h.res.Body).Decode(&out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}
