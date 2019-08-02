// Copyright (C) 2017 Jan Delgado

package main

import (
	"context"
	"crypto/tls"

	rabtap "github.com/jandelgado/rabtap/pkg"
)

func tapCmdShutdownFunc(taps []*rabtap.AmqpTap) {
	log.Info("rabtap tap threads shutting down ...")
	for _, tap := range taps {
		if !tap.Connected() {
			continue
		}
		if err := tap.Close(); err != nil {
			log.Errorf("error closing tap: %v", err)
		}
	}
}

// cmdTap taps to the given exchanges and displays or saves the received
// messages.
func cmdTap(ctx context.Context, tapConfig []rabtap.TapConfiguration, tlsConfig *tls.Config,
	messageReceiveFunc MessageReceiveFunc) {

	// this channel is used to decouple message receiving threads
	// with the main thread, which does the actual message processing
	tapMessageChannel := make(rabtap.TapChannel)
	taps := establishTaps(tapMessageChannel, tapConfig, tlsConfig)
	defer tapCmdShutdownFunc(taps)
	err := messageReceiveLoop(ctx, tapMessageChannel, messageReceiveFunc)
	if err != nil {
		log.Errorf("tap failed with %v", err)
	}

}

// establishTaps establishes all message taps as specified by tapConfiguration
// array. All received messages will be send to the provided tapMessageChannel
// channel. Returns array of tabtap.AmqpTap objects and immeadiately starts
// the processing.
// TODO feature: discover bindings when no binding keys are given (-> discovery.go)
func establishTaps(tapMessageChannel rabtap.TapChannel,
	tapConfigs []rabtap.TapConfiguration, tlsConfig *tls.Config) []*rabtap.AmqpTap {
	taps := []*rabtap.AmqpTap{}
	for _, config := range tapConfigs {
		tap := rabtap.NewAmqpTap(config.AmqpURI, tlsConfig, log)
		go tap.EstablishTap(config.Exchanges, tapMessageChannel)
		taps = append(taps, tap)
	}
	return taps
}
