# Go SDK for Forq - Simple Message Queue powered by SQLite

[![Go Reference](https://pkg.go.dev/badge/github.com/n0rdy/forq-sdk-go.svg)](https://pkg.go.dev/github.com/n0rdy/forq-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/n0rdy/forq-sdk-go)](https://goreportcard.com/report/github.com/n0rdy/forq-sdk-go)

Check out the [Forq project](https://forq.sh) for more information about the server itself.

## Go SDK

The Go SDK is available at [GitHub](https://github.com/n0rdy/forq-sdk-go)

```bash
go get github.com/n0rdy/forq-sdk-go
```

### Producer

You can create a new producer by providing HTTP client, Forq server URL and auth secret:

```go
httpClient := &http.Client{} // add necessary timeouts, etc., DO NOT use http.DefaultClient as it is not safe for production
forqURL := "http://localhost:8080"
authSecret := "your-auth-secret-min-32-chars-long"

p := producer.NewForqProducer(httpClient, forqURL, authSecret)
```

You can then use the producer to send messages:

```go
queueName := "my-queue"
newMessage := api.NewMessageRequest{
    Content: "I am going on an adventure!",
    ProcessAfter: time.Now().Add(time.Hour).UnixMilli(), // optional: deliver in 1 hour; omit for immediate delivery
}

err := p.Produce(context.Background(), newMessage, queueName)
```

### Consumer

You can create a new consumer by providing HTTP client, Forq server URL and auth secret:

```go
httpClient := &http.Client{} // add necessary timeouts, etc., DO NOT use http.DefaultClient as it is not safe for production
forqURL := "http://localhost:8080"
authSecret := "your-auth-secret-min-32-chars-long"

c, err := consumer.NewForqConsumer(httpClient, forqURL, authSecret)
// err is possible here if the provided HTTP Client timeout is shorter than 35 seconds (30 seconds long polling + 5 seconds buffer) 
```

Consumer provides a simple `ConsumeOne` function that will fetch one message.
It's up to you to build a consumption loop or goroutine pool to process messages concurrently.

Here is a simple consumption of 1 message:

```go
msg, err := c.ConsumeOne(context.Background(), "my-queue")
```

Then you'll process the message.
If processing is successful, you have to acknowledge the message, otherwise it will be re-delivered after the max processing time.

```go
err = c.Ack(context.Background(), "my-queue", msg)
```

If processing failed, you have to nack the message:

```go
err = c.Nack(context.Background(), "my-queue", msg)
```

`Ack` and `Nack` take the whole message (not just the ID) because the server requires the delivery receipt
from the consume response: it fences the ack/nack to that exact delivery, so a late ack/nack from a consumer
that exceeded the max processing time cannot affect a redelivery owned by another consumer. The SDK sends
the receipt for you via the `X-Forq-Receipt` header.