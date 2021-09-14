package main

import (
	"context"
	"fmt"

	"github.com/streadway/amqp"
)

func publishAmqp(ctx context.Context, amqpURL, amqpQueue string, data []byte) (err error) {
	var conn *amqp.Connection
	var ch *amqp.Channel
	var q amqp.Queue

	if conn, err = amqp.Dial(amqpURL); err != nil {
		err = fmt.Errorf("failed to connect to amqp broker: %w", err)
		return
	}
	defer conn.Close()

	if ch, err = conn.Channel(); err != nil {
		err = fmt.Errorf("failed to open a channel: %w", err)
		return
	}
	defer ch.Close()

	if q, err = ch.QueueDeclare(amqpQueue, false, true, false, true, nil); err != nil {
		err = fmt.Errorf("failed to declare a queue: %w", err)
		return
	}

	err = ch.Publish("", q.Name, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         data,
	})
	if err != nil {
		err = fmt.Errorf("failed publish a message: %w", err)
		return
	}

	return
}
