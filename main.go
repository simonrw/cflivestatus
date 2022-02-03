package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

// client is the interface that we consume from the AWS service.
type client interface {
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out: os.Stderr,
	})
	log.Debug().Msgf("%s starting", os.Args[0])

	var opts struct {
	}

	args, err := flags.Parse(&opts)
	if err != nil {
		log.Error().Msg("command line arguments invalid")
		os.Exit(1)
	}

	if len(args) == 0 {
		log.Error().Msg("no stack name specified")
		fatal("no stack name specified\n")
	}
	name := args[0]
	_ = name

	// TODO: update default region
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("eu-west-2"))
	if err != nil {
		log.Err(err).Msg("error loading default config")
	}

	svc := cloudformation.NewFromConfig(cfg)
	_ = svc
}
