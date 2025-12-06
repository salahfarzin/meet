package rabbitmq

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRabbitMQClient_InvalidURL(t *testing.T) {
	// Test with invalid URL - should return error
	client, err := NewRabbitMQClient("invalid-url")
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewRabbitMQClient_EmptyURL(t *testing.T) {
	// Test with empty URL - should return error
	client, err := NewRabbitMQClient("")
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestRabbitMQClient_Close(t *testing.T) {
	// Test Close method - should not panic even with nil connection/channel
	client := &RabbitMQClient{}
	// This should not panic
	client.Close()
}

func TestRabbitMQClient_MethodsWithNilChannel(t *testing.T) {
	// Test methods with nil channel - should panic or handle gracefully
	client := &RabbitMQClient{}

	// Publish with nil channel should panic
	assert.Panics(t, func() {
		_ = client.Publish("test-queue", []byte("test"))
	})

	// Consume with nil channel should panic
	assert.Panics(t, func() {
		_, _ = client.Consume("test-queue")
	})
}
