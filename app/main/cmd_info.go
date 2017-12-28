// Copyright (C) 2017 Jan Delgado

package main

import (
	"io"
	"os"

	"github.com/jandelgado/rabtap"
)

// CmdInfoArg contains arguments for the info command
type CmdInfoArg struct {
	rootNode              string
	client                *rabtap.RabbitHTTPClient
	printBrokerInfoConfig PrintBrokerInfoConfig
	out                   io.Writer
}

// cmdInfo queries the rabbitMQ brokers REST api and dispays infos
// on exchanges, queues, bindings etc in a human readably fashion.
func cmdInfo(cmd CmdInfoArg) {
	brokerInfo, err := NewBrokerInfo(cmd.client)
	failOnError(err, "failed retrieving info from rabbitmq REST api", os.Exit)
	brokerInfoPrinter := NewBrokerInfoPrinter(cmd.printBrokerInfoConfig)
	brokerInfoPrinter.Print(brokerInfo, cmd.rootNode, cmd.out)
}
