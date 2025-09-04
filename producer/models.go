package producer

import "forq-sdk-go/api"

type AsyncProducerMessage struct {
	NewMessage api.NewMessageRequest
	QueueName  string
}
