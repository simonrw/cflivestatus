package fetcher

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
)

// client is the interface that we consume from the AWS service.
type client interface {
	DescribeStackResources(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error)
}
