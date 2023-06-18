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
