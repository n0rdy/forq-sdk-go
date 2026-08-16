package producer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/n0rdy/forq-sdk-go/api"
)

const testSecret = "test-secret-that-is-32-chars-long"

func TestProduce_SendsCorrectRequest(t *testing.T) {
	var gotMethod, gotPath, gotAPIKey, gotContentType string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-API-Key")
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := NewForqProducer(&http.Client{}, srv.URL, testSecret)
	msg := api.NewMessageRequest{Content: "hello", ProcessAfter: 1755366229123}
	if err := p.Produce(context.Background(), msg, "orders"); err != nil {
		t.Fatal(err)
	}

	if gotMethod != http.MethodPost || gotPath != "/api/v1/queues/orders/messages" {
		t.Fatalf("request: %s %s", gotMethod, gotPath)
	}
	if gotAPIKey != testSecret {
		t.Fatalf("X-API-Key = %q", gotAPIKey)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q", gotContentType)
	}

	var sent api.NewMessageRequest
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Content != "hello" || sent.ProcessAfter != 1755366229123 {
		t.Fatalf("sent body: %+v", sent)
	}
}

func TestProduce_OmitsZeroProcessAfter(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := NewForqProducer(&http.Client{}, srv.URL, testSecret)
	if err := p.Produce(context.Background(), api.NewMessageRequest{Content: "x"}, "orders"); err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal(gotBody, &raw); err != nil {
		t.Fatal(err)
	}
	if _, present := raw["processAfter"]; present {
		t.Fatalf("processAfter should be omitted when zero, body: %s", gotBody)
	}
}

func TestProduce_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"code":"bad_request.queue.produce_to_dlq"}`))
	}))
	defer srv.Close()

	p := NewForqProducer(&http.Client{}, srv.URL, testSecret)
	err := p.Produce(context.Background(), api.NewMessageRequest{Content: "x"}, "orders-dlq")

	var errResp *api.ErrorResponse
	if !errors.As(err, &errResp) || errResp.Code != api.ErrCodeBadRequestProduceToDlq {
		t.Fatalf("got %v, want ErrorResponse{produce_to_dlq}", err)
	}
}
