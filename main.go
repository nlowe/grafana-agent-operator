package main

import (
	"os"

	"github.com/mattn/go-colorable"
	"github.com/nlowe/grafana-agent-operator/cmd"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

func main() {
	logrus.SetOutput(colorable.NewColorableStdout())
	logrus.SetFormatter(&prefixed.TextFormatter{
		ForceColors:     true,
		ForceFormatting: true,
		FullTimestamp:   true,
	})

	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
