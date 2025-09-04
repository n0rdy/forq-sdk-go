package producer

import (
	"net/http"
	"sync"
)

// AsyncForqProducer provides asynchronous message production to Forq via channels
type AsyncForqProducer struct {
	syncProducer *SyncForqProducer
	producerChan <-chan AsyncProducerMessage
	errChan      chan error
	stopChan     chan struct{}
	doneChan     chan struct{}
	stopOnce     sync.Once
}

func NewAsyncForqProducer(
	httpClient *http.Client,
	forqServerUrl string,
	authSecret string,
	producerChan <-chan AsyncProducerMessage,
	errChan chan error,
) *AsyncForqProducer {
	return &AsyncForqProducer{
		syncProducer: NewSyncForqProducer(httpClient, forqServerUrl, authSecret),
		producerChan: producerChan,
		errChan:      errChan,
		stopChan:     make(chan struct{}),
		doneChan:     make(chan struct{}),
	}
}

func (p *AsyncForqProducer) Start() {
	go func() {
		defer close(p.doneChan)

		for {
			select {
			case msg, ok := <-p.producerChan:
				if !ok {
					// producerChan was closed, time to exit
					return
				}
				err := p.syncProducer.Produce(msg.NewMessage, msg.QueueName)
				if err != nil {
					select {
					case p.errChan <- err:
					case <-p.stopChan:
						// if we can't send error, and we're stopping, just exit
						return
					}
				}
			case <-p.stopChan:
				// drain remaining messages to prevent message loss
				p.drainMessages()
				return
			}
		}
	}()
}

func (p *AsyncForqProducer) drainMessages() {
	for {
		select {
		case msg, ok := <-p.producerChan:
			if !ok {
				return
			}
			err := p.syncProducer.Produce(msg.NewMessage, msg.QueueName)
			if err != nil {
				// try to send error, but don't block if errChan is full
				select {
				case p.errChan <- err:
				default:
					// can't send error, continue draining
				}
			}
		default:
			// no more messages to drain
			return
		}
	}
}

func (p *AsyncForqProducer) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopChan)
	})
}

func (p *AsyncForqProducer) Wait() {
	<-p.doneChan
}

func (p *AsyncForqProducer) Close() error {
	p.Stop()
	p.Wait()
	return nil
}
