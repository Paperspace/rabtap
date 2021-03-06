// Copyright (C) 2017 Jan Delgado

package rabtap

import (
	"crypto/tls"
	"time"

	uuid "github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

// AmqpSubscriberConfig stores configuration of the subscriber
type AmqpSubscriberConfig struct {
	Exclusive bool
	AutoAck   bool
}

// AmqpSubscriber allows to tap to subscribe to queues
type AmqpSubscriber struct {
	config     AmqpSubscriberConfig
	connection *AmqpConnector
	logger     logrus.StdLogger
}

// NewAmqpSubscriber returns a new AmqpSubscriber object associated with the
// RabbitMQ broker denoted by the uri parameter.
func NewAmqpSubscriber(config AmqpSubscriberConfig, uri string, tlsConfig *tls.Config, logger logrus.StdLogger) *AmqpSubscriber {
	return &AmqpSubscriber{
		config:     config,
		connection: NewAmqpConnector(uri, tlsConfig, logger),
		logger:     logger}
}

// TapMessage objects are passed through a tapChannel from tap to client
// either AmqpMessage or Error is set
type TapMessage struct {
	AmqpMessage       *amqp.Delivery
	Error             error
	ReceivedTimestamp time.Time
}

// NewTapMessage constructs a new TapMessage
func NewTapMessage(message *amqp.Delivery, err error, ts time.Time) TapMessage {
	return TapMessage{AmqpMessage: message, Error: err, ReceivedTimestamp: ts}
}

// TapChannel is a channel for *TapMessage objects
type TapChannel chan TapMessage

// Close closes the connection to the broker and ends tapping. Returns result
// of amqp.Connection.Close() operation.
func (s *AmqpSubscriber) Close() error {
	return s.connection.Close()
}

// Connected returns true if the tap is connected to an exchange, otherwise
// false
func (s *AmqpSubscriber) Connected() bool {
	return s.connection.Connected()
}

// EstablishSubscription sets up the connection to the broker and sets up
// the tap, which is bound to the provided consumer function. Typically
// this function is run as a go-routine.
func (s *AmqpSubscriber) EstablishSubscription(queueName string, tapCh TapChannel) error {
	err := s.connection.Connect(s.createWorkerFunc(queueName, tapCh))
	if err != nil {
		tapCh <- NewTapMessage(nil, err, time.Now())
	}
	return err
}

func (s *AmqpSubscriber) createWorkerFunc(
	queueName string, tapCh TapChannel) AmqpWorkerFunc {

	return func(rabbitConn *amqp.Connection, controlCh chan ControlMessage) ReconnectAction {
		ch, err := s.consumeMessages(rabbitConn, queueName)
		if err != nil {
			tapCh <- NewTapMessage(nil, err, time.Now())
			return doNotReconnect
		}
		// messageloop expects Fanin object, which expects array of channels.
		var channels []interface{}
		fanin := NewFanin(append(channels, ch))
		return s.messageLoop(tapCh, fanin, controlCh)
	}
}

// messageLoop forwards incoming amqp messages from the fanin to the provided
// tapCh.
func (s *AmqpSubscriber) messageLoop(tapCh TapChannel,
	fanin *Fanin, controlCh <-chan ControlMessage) ReconnectAction {

	for {
		select {
		case message := <-fanin.Ch:
			amqpMessage, _ := message.(amqp.Delivery)
			tapCh <- NewTapMessage(&amqpMessage, nil, time.Now())

		case controlMessage := <-controlCh:
			switch controlMessage {
			case shutdownMessage:
				s.logger.Printf("AmqpSubscriber: shutdown")
				return doNotReconnect
			case reconnectMessage:
				s.logger.Printf("AmqpSubscriber: ending worker due to reconnect")
				return doReconnect
			}
		}
	}
}

func (s *AmqpSubscriber) consumeMessages(conn *amqp.Connection,
	queueName string) (<-chan amqp.Delivery, error) {

	var ch *amqp.Channel
	var err error

	if ch, err = conn.Channel(); err != nil {
		return nil, err
	}

	msgs, err := ch.Consume(
		queueName,
		"__rabtap-consumer-"+uuid.Must(uuid.NewRandom()).String()[:8], // TODO param
		s.config.AutoAck,
		s.config.Exclusive,
		false, // no-local - unsupported
		false, // wait
		nil,   // args
	)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}
