// Copyright (C) 2017 Jan Delgado

// +build integration

package main

// cmd_{exchangeCreate, sub, queueCreate, queueBind, queueDelete}
// integration test

import (
	"crypto/tls"
	"io"
	"os"
	"testing"
	"time"

	rabtap "github.com/jandelgado/rabtap/pkg"
	"github.com/jandelgado/rabtap/pkg/testcommon"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

func TestCmdSubFailsEarlyWhenBrokerIsNotAvailable(t *testing.T) {

	done := make(chan bool)
	go func() {
		cmdSubscribe(CmdSubscribeArg{
			amqpURI:            "invalid uri",
			queue:              "queue",
			tlsConfig:          &tls.Config{},
			messageReceiveFunc: func(rabtap.TapMessage) error { return nil },
			signalChannel:      make(chan os.Signal, 1)})
		done <- true
	}()

	// test if our tap received the message
	select {
	case <-done:
	case <-time.After(time.Second * 2):
		assert.Fail(t, "cmdSubscribe did not fail on initial connection error")
	}
}

func TestCmdSub(t *testing.T) {
	const testMessage = "SubHello"
	const testQueue = "sub-queue-test"
	const testKey = testQueue
	const testExchange = "sub-exchange-test"
	tlsConfig := &tls.Config{}
	amqpURI := testcommon.IntegrationURIFromEnv()

	done := make(chan bool)
	receiveFunc := func(message rabtap.TapMessage) error {
		log.Debug("test: received message: #+v", message)
		if string(message.AmqpMessage.Body) == testMessage {
			done <- true
		}
		return nil
	}

	// signalChannel receives ctrl+C/interrput signal
	signalChannel := make(chan os.Signal, 1)

	cmdExchangeCreate(CmdExchangeCreateArg{amqpURI: amqpURI,
		exchange: testExchange, exchangeType: "fanout",
		durable: false, tlsConfig: tlsConfig})
	defer cmdExchangeRemove(amqpURI, testExchange, tlsConfig)

	// create and bind queue
	cmdQueueCreate(CmdQueueCreateArg{amqpURI: amqpURI,
		queue: testQueue, tlsConfig: tlsConfig})
	cmdQueueBindToExchange(amqpURI, testQueue, testKey, testExchange, tlsConfig)
	defer cmdQueueRemove(amqpURI, testQueue, tlsConfig)

	// subscribe to testQueue
	go cmdSubscribe(CmdSubscribeArg{
		amqpURI:            amqpURI,
		queue:              testQueue,
		tlsConfig:          tlsConfig,
		messageReceiveFunc: receiveFunc,
		signalChannel:      signalChannel})

	time.Sleep(time.Second * 1)

	messageCount := 0
	cmdPublish(CmdPublishArg{
		amqpURI:    amqpURI,
		exchange:   testExchange,
		routingKey: testKey,
		tlsConfig:  tlsConfig,
		readNextMessageFunc: func() (amqp.Publishing, bool, error) {
			// provide exactly one message
			if messageCount > 0 {
				return amqp.Publishing{}, false, io.EOF
			}
			messageCount++
			return amqp.Publishing{
				Body:         []byte(testMessage),
				ContentType:  "text/plain",
				DeliveryMode: amqp.Transient,
			}, true, nil
		}})

	// test if our tap received the message
	select {
	case <-done:
	case <-time.After(time.Second * 2):
		assert.Fail(t, "did not receive message within expected time")
	}
	signalChannel <- os.Interrupt

	cmdQueueUnbindFromExchange(amqpURI, testQueue, testKey, testExchange, tlsConfig)
	// TODO check that queue is unbound
}
