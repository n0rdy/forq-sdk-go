package consumer

import (
	"encoding/json"
	"fmt"
	"forq-sdk-go/api"
	"net/http"
	"strings"
)

const (
	consumeMessageEndpointUrlTemplate = "/api/v1/queues/%s/messages"
	ackMessageEndpointUrlTemplate     = "/api/v1/queues/%s/messages/%s/ack"
	nackMessageEndpointUrlTemplate    = "/api/v1/queues/%s/messages/%s/nack"

	// max time of long polling by Forq server.
	// Forq uses long polling to wait for new messages to arrive if the queue is empty.
	// This is to avoid constant polling from the client side when the queue is empty.
	// The client should set its HTTP client timeout to be at least this value + few seconds extra buffer on top.
	longPollingMaxDurationSec = 30
)

var (
	HttpClientTimeoutTooShortError = fmt.Errorf("http client timeout must be 0 (no timeout) or at least (%d + few seconds extra buffer on top) seconds", longPollingMaxDurationSec)
)

type SyncForqConsumer struct {
	httpClient    *http.Client
	forqServerUrl string
	apiKeyHeader  string
}

func NewSyncForqConsumer(
	httpClient *http.Client,
	forqServerUrl string,
	authSecret string,
) (*SyncForqConsumer, error) {
	if strings.HasSuffix(forqServerUrl, "/") {
		forqServerUrl = strings.TrimSuffix(forqServerUrl, "/")
	}
	if httpClient.Timeout != 0 && httpClient.Timeout.Seconds() < longPollingMaxDurationSec {
		return nil, HttpClientTimeoutTooShortError
	}

	return &SyncForqConsumer{
		httpClient:    httpClient,
		forqServerUrl: forqServerUrl,
		apiKeyHeader:  "ApiKey " + authSecret,
	}, nil
}

func (c *SyncForqConsumer) ConsumeOne(queueName string) (*api.MessageResponse, error) {
	endpoint := fmt.Sprintf(c.forqServerUrl+consumeMessageEndpointUrlTemplate, queueName)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", c.apiKeyHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		// no message available
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		return nil, &errResp
	}

	var message api.MessageResponse
	err = json.NewDecoder(resp.Body).Decode(&message)
	if err != nil {
		return nil, fmt.Errorf("failed to decode message response: %w", err)
	}
	return &message, nil
}

func (c *SyncForqConsumer) Ack(queueName string, messageId string) error {
	endpoint := fmt.Sprintf(c.forqServerUrl+ackMessageEndpointUrlTemplate, queueName, messageId)

	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", c.apiKeyHeader)

	resp, err := c.httpClient.Do(req)
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

func (c *SyncForqConsumer) Nack(queueName string, messageId string) error {
	endpoint := fmt.Sprintf(c.forqServerUrl+nackMessageEndpointUrlTemplate, queueName, messageId)

	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", c.apiKeyHeader)

	resp, err := c.httpClient.Do(req)
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
