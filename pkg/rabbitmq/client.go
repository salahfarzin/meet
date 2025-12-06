package rabbitmq

import (
	"github.com/streadway/amqp"
)

type RabbitMQClient struct {
	connection *amqp.Connection
	channel    *amqp.Channel
}

func NewRabbitMQClient(url string) (*RabbitMQClient, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return &RabbitMQClient{
		connection: conn,
		channel:    ch,
	}, nil
}

func (r *RabbitMQClient) Publish(queueName string, body []byte) error {
	_, err := r.channel.QueueDeclare(
		queueName,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	err = r.channel.Publish(
		"",
		queueName,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	return err
}

func (r *RabbitMQClient) Consume(queueName string) (<-chan amqp.Delivery, error) {
	msgs, err := r.channel.Consume(
		queueName,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	return msgs, err
}

func (r *RabbitMQClient) Close() {
	if r.channel != nil {
		_ = r.channel.Close() // Ignore close errors
	}
	if r.connection != nil {
		_ = r.connection.Close() // Ignore close errors
	}
}
