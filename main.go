/*
 * Copyright (c) AppDynamics, Inc., and its affiliates 2020
 * All Rights Reserved.
 * THIS IS UNPUBLISHED PROPRIETARY CODE OF APPDYNAMICS, INC.
 *
 * The copyright notice above does not evidence any actual or
 * intended publication of such source code
 */

package main

import (
	"flag"
	"go.uber.org/zap"
	"in-cluster/internal/agent"
	"os"
	"os/signal"
)

func main() {
	var agentType string
	flag.StringVar(&agentType, "t", "io.opentelemetry.collector", "Agent Type String")

	var agentVersion string
	flag.StringVar(&agentVersion, "v", "1.0.0", "Agent Version String")

	flag.Parse()
	sugar := zap.NewExample().Sugar()
	defer sugar.Sync()
	opamoAgent := agent.NewAgent(sugar, agentType, agentVersion)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	opamoAgent.Shutdown()
}
