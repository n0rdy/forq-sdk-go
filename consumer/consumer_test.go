package consumer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/n0rdy/forq-sdk-go/api"
)

const testSecret = "test-secret-that-is-32-chars-long"

type recordedRequest struct {
	method  string
	path    string
	apiKey  string
	receipt string
}

// newServer returns an httptest server that records the last request and
// replies with the given status and body.
func newServer(t *testing.T, status int, body string) (*httptest.Server, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.apiKey = r.Header.Get("X-API-Key")
		rec.receipt = r.Header.Get("X-Forq-Receipt")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func newConsumer(t *testing.T, serverURL string) *ForqConsumer {
	t.Helper()
	c, err := NewForqConsumer(&http.Client{}, serverURL, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewForqConsumer_TimeoutValidation(t *testing.T) {
	// too short for the 30s long poll
	if _, err := NewForqConsumer(&http.Client{Timeout: 5 * time.Second}, "http://x", testSecret); !errors.Is(err, HttpClientTimeoutTooShortError) {
		t.Fatalf("5s timeout: got %v, want HttpClientTimeoutTooShortError", err)
	}
	// no timeout is fine
	if _, err := NewForqConsumer(&http.Client{}, "http://x", testSecret); err != nil {
		t.Fatalf("no timeout: %v", err)
	}
	// long enough is fine
	if _, err := NewForqConsumer(&http.Client{Timeout: 40 * time.Second}, "http://x", testSecret); err != nil {
		t.Fatalf("40s timeout: %v", err)
	}
}

func TestConsumeOne_ParsesMessageWithReceipt(t *testing.T) {
	srv, rec := newServer(t, http.StatusOK, `{"id":"msg-1","content":"hello","receipt":"1755366229123"}`)
	c := newConsumer(t, srv.URL)

	msg, err := c.ConsumeOne(context.Background(), "orders")
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID != "msg-1" || msg.Content != "hello" || msg.Receipt != "1755366229123" {
		t.Fatalf("parsed message: %+v", msg)
	}
	if rec.method != http.MethodGet || rec.path != "/api/v1/queues/orders/messages" {
		t.Fatalf("request: %s %s", rec.method, rec.path)
	}
	if rec.apiKey != testSecret {
		t.Fatalf("X-API-Key = %q", rec.apiKey)
	}
}

func TestConsumeOne_NoMessage(t *testing.T) {
	srv, _ := newServer(t, http.StatusNoContent, "")
	c := newConsumer(t, srv.URL)

	msg, err := c.ConsumeOne(context.Background(), "orders")
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatalf("expected nil message on 204, got %+v", msg)
	}
}

func TestConsumeOne_ErrorResponse(t *testing.T) {
	srv, _ := newServer(t, http.StatusUnauthorized, `{"code":"unauthorized"}`)
	c := newConsumer(t, srv.URL)

	_, err := c.ConsumeOne(context.Background(), "orders")
	var errResp *api.ErrorResponse
	if !errors.As(err, &errResp) || errResp.Code != api.ErrCodeUnauthorized {
		t.Fatalf("got %v, want ErrorResponse{unauthorized}", err)
	}
}

func TestAck_SendsReceiptHeader(t *testing.T) {
	srv, rec := newServer(t, http.StatusNoContent, "")
	c := newConsumer(t, srv.URL)

	msg := &api.MessageResponse{ID: "msg-1", Content: "x", Receipt: "1755366229123"}
	if err := c.Ack(context.Background(), "orders", msg); err != nil {
		t.Fatal(err)
	}

	if rec.method != http.MethodPost || rec.path != "/api/v1/queues/orders/messages/msg-1/ack" {
		t.Fatalf("request: %s %s", rec.method, rec.path)
	}
	if rec.receipt != "1755366229123" {
		t.Fatalf("X-Forq-Receipt = %q", rec.receipt)
	}
}

func TestNack_SendsReceiptHeader(t *testing.T) {
	srv, rec := newServer(t, http.StatusNoContent, "")
	c := newConsumer(t, srv.URL)

	msg := &api.MessageResponse{ID: "msg-1", Content: "x", Receipt: "1755366229123"}
	if err := c.Nack(context.Background(), "orders", msg); err != nil {
		t.Fatal(err)
	}

	if rec.path != "/api/v1/queues/orders/messages/msg-1/nack" {
		t.Fatalf("path: %s", rec.path)
	}
	if rec.receipt != "1755366229123" {
		t.Fatalf("X-Forq-Receipt = %q", rec.receipt)
	}
}

func TestAck_StaleReceiptSurfacesNotFound(t *testing.T) {
	srv, _ := newServer(t, http.StatusNotFound, `{"code":"not_found.message"}`)
	c := newConsumer(t, srv.URL)

	msg := &api.MessageResponse{ID: "msg-1", Receipt: "stale"}
	err := c.Ack(context.Background(), "orders", msg)
	var errResp *api.ErrorResponse
	if !errors.As(err, &errResp) || errResp.Code != api.ErrCodeNotFoundMessage {
		t.Fatalf("got %v, want ErrorResponse{not_found.message}", err)
	}
}

func TestTrailingSlashInServerURL(t *testing.T) {
	srv, rec := newServer(t, http.StatusNoContent, "")
	c := newConsumer(t, srv.URL+"/")

	c.ConsumeOne(context.Background(), "orders")
	if rec.path != "/api/v1/queues/orders/messages" {
		t.Fatalf("trailing slash not trimmed, path: %s", rec.path)
	}
}

func TestAckNack_NilMessageReturnsError(t *testing.T) {
	srv, _ := newServer(t, http.StatusNoContent, "")
	c := newConsumer(t, srv.URL)

	if err := c.Ack(context.Background(), "orders", nil); !errors.Is(err, NilMessageError) {
		t.Fatalf("Ack(nil): got %v, want NilMessageError", err)
	}
	if err := c.Nack(context.Background(), "orders", nil); !errors.Is(err, NilMessageError) {
		t.Fatalf("Nack(nil): got %v, want NilMessageError", err)
	}
}

func TestQueueNameIsPathEscaped(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.URL.RequestURI()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	c := newConsumer(t, srv.URL)

	// a queue name with a slash must not change the request path shape
	c.ConsumeOne(context.Background(), "orders/evil")
	if gotRequestURI != "/api/v1/queues/orders%2Fevil/messages" {
		t.Fatalf("queue name not path-escaped, request URI: %s", gotRequestURI)
	}
}
