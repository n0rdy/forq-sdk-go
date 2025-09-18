package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/n0rdy/forq-sdk-go/api"
)

const (
	produceMessageEndpointUrlTemplate = "/api/v1/queues/%s/messages"
)

// ForqProducer provides message production to Forq
type ForqProducer struct {
	httpClient    *http.Client
	forqServerUrl string
	authSecret    string
}

func NewForqProducer(
	httpClient *http.Client,
	forqServerUrl string,
	authSecret string,
) *ForqProducer {
	if strings.HasSuffix(forqServerUrl, "/") {
		forqServerUrl = strings.TrimSuffix(forqServerUrl, "/")
	}
	return &ForqProducer{
		httpClient:    httpClient,
		forqServerUrl: forqServerUrl,
		authSecret:    authSecret,
	}
}

func (p *ForqProducer) Produce(
	context context.Context,
	newMessage api.NewMessageRequest,
	queueName string,
) error {
	reqBody, err := json.Marshal(newMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal new message request: %w", err)
	}

	endpoint := fmt.Sprintf(p.forqServerUrl+produceMessageEndpointUrlTemplate, queueName)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(reqBody)))
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", p.authSecret)

	resp, err := p.httpClient.Do(req.WithContext(context))
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	var errResp api.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		return fmt.Errorf("failed to decode error response: %w", err)
	}
	return &errResp
}
