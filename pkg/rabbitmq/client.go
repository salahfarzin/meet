package rabbitmq

import (
    "github.com/streadway/amqp"
    "log"
)

type RabbitMQClient struct {
    connection *amqp.Connection
    channel    *amqp.Channel
}

func NewRabbitMQClient(url string) (*RabbitMQClient, error) {
    conn, err := amqp.Dial(url)
    if err != nil {
        log.Fatalf("Failed to connect to RabbitMQ: %s", err)
        return nil, err
    }

    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("Failed to open a channel: %s", err)
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
    r.channel.Close()
    r.connection.Close()
}