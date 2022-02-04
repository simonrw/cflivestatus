package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/smithy-go"
	"github.com/jessevdk/go-flags"
	"github.com/mindriot101/cflivestatus/fetcher"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out: os.Stderr,
	})

	ctx := context.TODO()

	var opts struct {
		Verbose []bool `short:"v" long:"verbose" description:"Print verbose logging output"`
	}

	args, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}
	switch len(opts.Verbose) {
	case 0:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Debug().Msgf("%s starting", os.Args[0])
	log.Debug().Interface("opts", opts).Msg("parsed command line options")

	if len(args) == 0 {
		log.Error().Msg("no stack name specified")
		fatal("no stack name specified\n")
	}
	name := args[0]
	_ = name

	// TODO: update default region
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("eu-west-2"))
	if err != nil {
		log.Err(err).Msg("error loading default config")
	}

	resourceStatuses := fetcher.NewResourceStatuses()

	svc := cloudformation.NewFromConfig(cfg)
	f := fetcher.New(name, svc)

	sleepTime := 2 * time.Second
	for {
		if err := f.UpdateResourceStatuses(ctx, resourceStatuses); err != nil {
			var oe *smithy.GenericAPIError
			if errors.As(err, &oe) {
				if oe.Message == fmt.Sprintf("Stack with id %s does not exist", name) {
					fatal("cannot find stack %s\n", name)
				}
			}
			log.Warn().Err(err).Msg("error when polling stack resources")
			time.Sleep(sleepTime)
			continue
		}
		presentState(resourceStatuses)
		time.Sleep(sleepTime)
	}
}

func presentState(statuses *fetcher.ResourceStatuses) {
	log.Info().Interface("statuses", statuses).Msg("got statuses")
}
