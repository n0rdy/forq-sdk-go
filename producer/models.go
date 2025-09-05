package producer

import "github.com/n0rdy/forq-sdk-go/api"

type AsyncProducerMessage struct {
	NewMessage api.NewMessageRequest
	QueueName  string
}
