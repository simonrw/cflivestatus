package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/smithy-go"
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
	DescribeStackResources(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error)
}

type resourceStatuses map[string]types.ResourceStatus

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

	resourceStatuses := make(resourceStatuses)

	svc := cloudformation.NewFromConfig(cfg)

	sleepTime := 2 * time.Second
	for {
		res, err := fetchResourceStatuses(ctx, name, svc)
		if err != nil {
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
		log.Debug().Interface("res", *res).Msg("got result")
		updateState(&resourceStatuses, res)
		presentState(&resourceStatuses)
		time.Sleep(sleepTime)
	}
}

func fetchResourceStatuses(ctx context.Context, stackName string, client client) (*cloudformation.DescribeStackResourcesOutput, error) {
	params := &cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(stackName),
	}
	return client.DescribeStackResources(ctx, params)
}

func updateState(statuses *resourceStatuses, res *cloudformation.DescribeStackResourcesOutput) {
	for _, resource := range res.StackResources {
		logicalName := resource.LogicalResourceId
		status := resource.ResourceStatus
		(*statuses)[*logicalName] = status
	}
}

func presentState(statuses *resourceStatuses) {
	log.Info().Interface("statuses", statuses).Msg("got statuses")
}
