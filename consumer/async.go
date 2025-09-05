package consumer

import (
	"net/http"
	"sync"

	"github.com/n0rdy/forq-sdk-go/api"
)

type AsyncForqConsumer struct {
	syncConsumer *SyncForqConsumer
	queueName    string
	consumerChan chan<- *api.MessageResponse
	ackChan      <-chan string
	nackChan     <-chan string
	errChan      chan<- error
	stopChan     chan struct{}
	doneChan     chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup
}

func NewAsyncForqConsumer(
	httpClient *http.Client,
	forqServerUrl string,
	authSecret string,
	queueName string,
	consumerChan chan<- *api.MessageResponse,
	ackChan <-chan string,
	nackChan <-chan string,
	errChan chan<- error,
) (*AsyncForqConsumer, error) {
	syncConsumer, err := NewSyncForqConsumer(httpClient, forqServerUrl, authSecret)
	if err != nil {
		return nil, err
	}

	return &AsyncForqConsumer{
		syncConsumer: syncConsumer,
		queueName:    queueName,
		consumerChan: consumerChan,
		ackChan:      ackChan,
		nackChan:     nackChan,
		errChan:      errChan,
		stopChan:     make(chan struct{}),
		doneChan:     make(chan struct{}),
	}, nil
}

// consumeLoop handles message consumption in a separate goroutine
func (c *AsyncForqConsumer) consumeLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		default:
			msg, err := c.syncConsumer.ConsumeOne(c.queueName)
			if err != nil {
				select {
				case c.errChan <- err:
				case <-c.stopChan:
					return
				}
				continue
			}
			if msg != nil {
				select {
				case c.consumerChan <- msg:
				case <-c.stopChan:
					return
				}
			}
		}
	}
}

// ackNackLoop handles ack/nack operations in a separate goroutine
func (c *AsyncForqConsumer) ackNackLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			// drain remaining acks/nacks before stopping
			c.drainAcksAndNacks()
			return
		case messageId, ok := <-c.ackChan:
			if !ok {
				// ackChan was closed, time to exit
				return
			}
			err := c.syncConsumer.Ack(c.queueName, messageId)
			if err != nil {
				select {
				case c.errChan <- err:
				case <-c.stopChan:
					return
				}
			}
		case messageId, ok := <-c.nackChan:
			if !ok {
				// nackChan was closed, time to exit
				return
			}
			err := c.syncConsumer.Nack(c.queueName, messageId)
			if err != nil {
				select {
				case c.errChan <- err:
				case <-c.stopChan:
					return
				}
			}
		}
	}
}

func (c *AsyncForqConsumer) Start() {
	// Start both goroutines
	c.wg.Add(2)

	// handles message consumption
	go c.consumeLoop()

	// handles acks/nacks
	go c.ackNackLoop()

	// Wait for both goroutines to complete, then close doneChan
	go func() {
		c.wg.Wait()
		close(c.doneChan)
	}()
}

func (c *AsyncForqConsumer) drainAcksAndNacks() {
	for {
		select {
		case messageId := <-c.ackChan:
			err := c.syncConsumer.Ack(c.queueName, messageId)
			if err != nil {
				// try to send error, but don't block if errChan is full
				select {
				case c.errChan <- err:
				default:
					// can't send error, continue draining
				}
			}
		case messageId := <-c.nackChan:
			err := c.syncConsumer.Nack(c.queueName, messageId)
			if err != nil {
				// try to send error, but don't block if errChan is full
				select {
				case c.errChan <- err:
				default:
					// can't send error, continue draining
				}
			}
		default:
			// no more acks/nacks to drain
			return
		}
	}
}

func (c *AsyncForqConsumer) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopChan)
	})
}

func (c *AsyncForqConsumer) Wait() {
	<-c.doneChan
}

func (c *AsyncForqConsumer) Close() error {
	c.Stop()
	c.Wait()
	return nil
}
