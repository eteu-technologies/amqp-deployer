package main

import (
	"fmt"

	"github.com/streadway/amqp"
)

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   amqp.Queue
	lastErr *amqp.Error

	Channel <-chan amqp.Delivery
}

func (c *Consumer) Err() *amqp.Error {
	return c.lastErr
}

func (c *Consumer) Close() (err error) {
	if c.lastErr != nil {
		// Nothing to do
		return
	}

	if err = c.channel.Close(); err != nil {
		return
	}
	if err = c.conn.Close(); err != nil {
		return
	}
	return
}

func setupConsumer(amqpURL, amqpQueue string) (consumer *Consumer, err error) {
	c := &Consumer{}

	if c.conn, err = amqp.Dial(amqpURL); err != nil {
		err = fmt.Errorf("failed to connect to amqp broker: %w", err)
		return
	}
	go func() {
		c.lastErr = <-c.conn.NotifyClose(make(chan *amqp.Error))
	}()

	if c.channel, err = c.conn.Channel(); err != nil {
		err = fmt.Errorf("failed to open channel: %w", err)
		return
	}

	if c.queue, err = c.channel.QueueDeclare(amqpQueue, false, true, false, true, nil); err != nil {
		err = fmt.Errorf("failed to declare a queue: %w", err)
		return
	}

	if err = c.channel.Qos(1, 0, false); err != nil {
		err = fmt.Errorf("failed to configure channel qos: %w", err)
		return
	}

	if c.Channel, err = c.channel.Consume(c.queue.Name, "", true, false, false, true, nil); err != nil {
		err = fmt.Errorf("failed to setup consumer: %w", err)
		return
	}

	consumer = c
	return
}
