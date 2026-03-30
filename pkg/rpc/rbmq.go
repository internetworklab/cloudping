package rpc

import (
	"context"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	pkgctx "github.com/internetworklab/cloudping/pkg/ctx"
	"github.com/google/uuid"
)

type RabbitMQRemoteCaller struct {
	RoutingKey string
}

type RBMQCallUpdate struct {
	Err     error
	Message []byte
}

func (rbmqRemoteCaller *RabbitMQRemoteCaller) Call(ctx context.Context, msgBody []byte) <-chan RBMQCallUpdate {
	evChan := make(chan RBMQCallUpdate)

	respondWithError := func(err error) {
		evChan <- RBMQCallUpdate{Err: err}
	}

	go func() {
		defer close(evChan)

		conn, err := pkgctx.GetRabbitMQConnection(ctx)
		if err != nil {
			respondWithError(fmt.Errorf("failed to obtain a RabbitMQ connection within RabbitMQPinger: %w", err))
			return
		}

		ch, err := conn.Channel()
		if err != nil {
			respondWithError(fmt.Errorf("failed to open a RabbitMQ channel within RabbitMQPinger: %w", err))
			return
		}
		defer ch.Close()

		// queue for reading RPC responses from the RPC server
		q, err := ch.QueueDeclare(
			"",    // queue name, generated randomly
			false, // durable
			true,  // delete when unused
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			respondWithError(fmt.Errorf("failed to declare a RabbitMQ queue within RabbitMQPinger: %w", err))
			return
		}

		// these 'msgs' are responses from the RPC server
		msgs, err := ch.Consume(
			q.Name, // queue
			"",     // consumer
			true,   // auto-ack
			false,  // exclusive
			false,  // no-local
			false,  // no-wait
			nil,    // arguments
		)

		if err != nil {
			respondWithError(fmt.Errorf("failed to register a consumer within RabbitMQPinger: %w", err))
			return
		}

		corrId := uuid.New().String()
		msgId := uuid.New().String()
		exchgName := ""

		err = ch.PublishWithContext(ctx,
			exchgName,                   // exchange
			rbmqRemoteCaller.RoutingKey, // routing key
			false,                       // mandatory
			false,                       // immediate
			amqp.Publishing{
				ContentType:   "application/json",
				CorrelationId: corrId,
				ReplyTo:       q.Name,
				Body:          msgBody,
				MessageId:     msgId,
			},
		)
		if err != nil {
			respondWithError(fmt.Errorf("failed to publish the message to the RabbitMQ exchange: %w", err))
			return
		}

		log.Println("Published a message", "exchg", exchgName, "routing_key", rbmqRemoteCaller.RoutingKey, "correlation_id", corrId, "message_id", msgId, "reply_to", q.Name)

		for msg := range msgs {
			log.Println("Received a message", "exchg", msg.Exchange, "routing_key", msg.RoutingKey, "correlation_id", msg.CorrelationId, "message_id", msg.MessageId)
			if msg.CorrelationId != corrId {
				log.Println("Received a message with a different correlation id", "exchg", msg.Exchange, "routing_key", msg.RoutingKey, "correlation_id", msg.CorrelationId, "message_id", msg.MessageId)
				continue
			}

			if len(msg.Body) == 0 {
				// A nill message body signals the end of the message stream
				break
			}

			evChan <- RBMQCallUpdate{Message: msg.Body}
		}
	}()

	return evChan
}
