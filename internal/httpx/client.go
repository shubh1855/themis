package httpx

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/security"
)

const (
	DefaultTimeout = 30 * time.Second
	MaxBodySize int64 = 10 * 1024 * 1024
	MaxRedirects = 10
)

type Client struct {
	http       *http.Client
	retrier    *Retrier
	limiter    *HostLimiter
	ssrf       *security.SSRFChecker
	userAgents []string
	uaIndex    int
}

type ClientOption func(*Client)

func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.http.Timeout = d
	}
}

func WithMaxRetries(n int) ClientOption {
	return func(c *Client) {
		c.retrier.MaxRetries = n
	}
}

func WithRateLimit(rps float64) ClientOption {
	return func(c *Client) {
		c.limiter.defaultRPS = rps
	}
}

func WithSSRFAllowPrivate() ClientOption {
	return func(c *Client) {
		c.ssrf.AllowPrivate = true
	}
}

func NewClient(opts ...ClientOption) *Client {
	transport := NewTransport()

	c := &Client{
		http: &http.Client{
			Timeout:   DefaultTimeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= MaxRedirects {
					return fmt.Errorf("httpx: too many redirects (%d)", MaxRedirects)
				}
				return nil
			},
		},
		retrier:    NewRetrier(3, 500*time.Millisecond, 10*time.Second),
		limiter:    NewHostLimiter(2.0),
		ssrf:       security.NewSSRFChecker(),
		userAgents: DefaultUserAgents(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.ssrf.Check(req.URL.String()); err != nil {
		return nil, err
	}

	if err := c.limiter.Wait(ctx, req.URL.Host); err != nil {
		return nil, fmt.Errorf("httpx: rate limit: %w", err)
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.nextUserAgent())
	}

	req.Header.Set("Accept-Encoding", "gzip")

	var resp *http.Response
	err := c.retrier.Do(ctx, func() error {
		var rerr error
		resp, rerr = c.http.Do(req.WithContext(ctx))
		if rerr != nil {
			return rerr
		}
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			return &RetryableError{StatusCode: resp.StatusCode}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("httpx: request failed: %w", err)
	}

	return resp, nil
}

func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("httpx: %w", err)
	}
	return c.Do(ctx, req)
}

func (c *Client) GetBody(ctx context.Context, url string) (string, error) {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gr, gerr := gzip.NewReader(resp.Body)
		if gerr != nil {
			return "", fmt.Errorf("httpx: gzip: %w", gerr)
		}
		defer gr.Close()
		reader = gr
	}

	limited := io.LimitReader(reader, MaxBodySize)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("httpx: read body: %w", err)
	}

	return string(body), nil
}

func (c *Client) nextUserAgent() string {
	ua := c.userAgents[c.uaIndex%len(c.userAgents)]
	c.uaIndex++
	return ua
}

type RetryableError struct {
	StatusCode int
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("httpx: server error %d", e.StatusCode)
}
