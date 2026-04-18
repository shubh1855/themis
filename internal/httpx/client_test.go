package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
)

func TestRetrier_SuccessOnFirstAttempt(t *testing.T) {
	r := httpx.NewRetrier(3, 10*time.Millisecond, 100*time.Millisecond)
	var calls int32

	err := r.Do(context.Background(), func() error {
		atomic.AddInt32(&calls, 1)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetrier_RetriesOnServerError(t *testing.T) {
	r := httpx.NewRetrier(2, 10*time.Millisecond, 100*time.Millisecond)
	var calls int32

	err := r.Do(context.Background(), func() error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return &httpx.RetryableError{StatusCode: 500}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetrier_ContextCancellation(t *testing.T) {
	r := httpx.NewRetrier(5, 100*time.Millisecond, 1*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := r.Do(ctx, func() error {
		return &httpx.RetryableError{StatusCode: 503}
	})

	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestRateLimiter_Basic(t *testing.T) {
	limiter := httpx.NewHostLimiter(100)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := limiter.Wait(ctx, "example.com"); err != nil {
			t.Fatalf("unexpected error on wait %d: %v", i, err)
		}
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	limiter := httpx.NewHostLimiter(0.001)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_ = limiter.Wait(ctx, "slow.example.com")

	err := limiter.Wait(ctx, "slow.example.com")
	if err == nil {
		t.Error("expected context deadline error")
	}
}

func TestClient_WithHTTPTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hello from test server"))
	}))
	defer server.Close()

	client := httpx.NewClient(httpx.WithSSRFAllowPrivate())
	body, err := client.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "hello from test server" {
		t.Errorf("expected 'hello from test server', got %q", body)
	}
}

func TestClient_RetriesOn500(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := httpx.NewClient(
		httpx.WithSSRFAllowPrivate(),
		httpx.WithMaxRetries(3),
	)

	body, err := client.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "ok" {
		t.Errorf("expected 'ok', got %q", body)
	}
}
