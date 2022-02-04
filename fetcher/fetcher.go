package fetcher

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
)

type fetcher struct {
	stackName string
	client    client
}

func New(stackName string, client client) *fetcher {
	return &fetcher{
		stackName,
		client,
	}
}

func (f *fetcher) fetch(ctx context.Context) ([]stackResource, error) {
	params := &cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(f.stackName),
	}
	res, err := f.client.DescribeStackResources(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("describing stack resources")
	}

	out := []stackResource{}
	for _, r := range res.StackResources {
		var resource string
		if r.LogicalResourceId != nil {
			resource = *r.LogicalResourceId
		} else {
			resource = "?"
		}

		out = append(out, stackResource{
			resource: resource,
			status:   r.ResourceStatus,
		})
	}

	return out, nil
}

func (f *fetcher) UpdateResourceStatuses(ctx context.Context, statuses *ResourceStatuses) error {
	resources, err := f.fetch(ctx)
	if err != nil {
		return err
	}
	for _, r := range resources {
		(*statuses)[r.resource] = r.status
	}
	return nil
}
