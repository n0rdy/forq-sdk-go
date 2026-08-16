package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/n0rdy/forq-sdk-go/api"
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
	longPollingBufferSec      = 5
)

var (
	HttpClientTimeoutTooShortError = fmt.Errorf("http client timeout must be 0 (no timeout) or at least (%d + few seconds extra buffer on top) seconds", longPollingMaxDurationSec)
	NilMessageError                = errors.New("message must not be nil - pass the message returned by ConsumeOne")
)

type ForqConsumer struct {
	httpClient    *http.Client
	forqServerUrl string
	authSecret    string
}

func NewForqConsumer(
	httpClient *http.Client,
	forqServerUrl string,
	authSecret string,
) (*ForqConsumer, error) {
	if httpClient.Timeout != 0 && httpClient.Timeout.Seconds() < (longPollingMaxDurationSec+longPollingBufferSec) {
		return nil, HttpClientTimeoutTooShortError
	}
	if strings.HasSuffix(forqServerUrl, "/") {
		forqServerUrl = strings.TrimSuffix(forqServerUrl, "/")
	}

	return &ForqConsumer{
		httpClient:    httpClient,
		forqServerUrl: forqServerUrl,
		authSecret:    authSecret,
	}, nil
}

func (c *ForqConsumer) ConsumeOne(
	context context.Context,
	queueName string,
) (*api.MessageResponse, error) {
	endpoint := fmt.Sprintf(c.forqServerUrl+consumeMessageEndpointUrlTemplate, url.PathEscape(queueName))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.authSecret)

	resp, err := c.httpClient.Do(req.WithContext(context))
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

// Ack acknowledges the given message as successfully processed. The message's
// Receipt fences the ack to this exact delivery: if the message exceeded the
// visibility timeout and was redelivered to another consumer, the ack returns
// a not_found.message error instead of affecting the other delivery.
func (c *ForqConsumer) Ack(
	context context.Context,
	queueName string,
	message *api.MessageResponse,
) error {
	if message == nil {
		return NilMessageError
	}
	endpoint := fmt.Sprintf(c.forqServerUrl+ackMessageEndpointUrlTemplate, url.PathEscape(queueName), message.ID)

	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.authSecret)
	req.Header.Set("X-Forq-Receipt", message.Receipt)

	resp, err := c.httpClient.Do(req.WithContext(context))
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

// Nack reports the given message as failed to process, scheduling a retry (or
// a DLQ move once attempts are exhausted). Like Ack, it is fenced to this
// exact delivery via the message's Receipt.
func (c *ForqConsumer) Nack(
	context context.Context,
	queueName string,
	message *api.MessageResponse,
) error {
	if message == nil {
		return NilMessageError
	}
	endpoint := fmt.Sprintf(c.forqServerUrl+nackMessageEndpointUrlTemplate, url.PathEscape(queueName), message.ID)

	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.authSecret)
	req.Header.Set("X-Forq-Receipt", message.Receipt)

	resp, err := c.httpClient.Do(req.WithContext(context))
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
