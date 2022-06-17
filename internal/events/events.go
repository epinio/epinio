// Package events handles everything related to the Epinio event system.
package events

import (
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/streadway/amqp"
)

func Send(queue, body string) error {
	// TODO: Should we just use one "global" connection or connect every time
	// we want to send something?
	conn, err := GetConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	if conn == nil {
		return nil
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queue, // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	return ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
}

func Receive(socketConn *websocket.Conn, queue string) error {
	conn, err := GetConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		queue, // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	// TODO: Consider returning some other type so that we don't expose
	// Rabbitmq to consumers
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return err
	}

	for {
		select {
		case d := <-msgs:
			err = socketConn.WriteMessage(websocket.TextMessage, d.Body)
			if err != nil {
				return err
			}
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func GetConnection() (*amqp.Connection, error) {
	pass := os.Getenv("RABBITMQ_PASSWORD")
	if pass == "" {
		return nil, nil
	}

	host := os.Getenv("RABBITMQ_HOST")
	if host == "" {
		return nil, nil
	}

	port := os.Getenv("RABBITMQ_PORT")
	if port == "" {
		return nil, nil
	}

	return amqp.Dial(fmt.Sprintf("amqp://user:%s@%s:%s/", pass, host, port))
}
