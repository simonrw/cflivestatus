package main

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/matryer/is"
)

type handlerFunc func(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error)

type mockClient struct {
	fn handlerFunc
}

func (m *mockClient) DescribeStackResources(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error) {
	if m.fn == nil {
		panic("missing function handler")
	}
	return m.fn(ctx, params, optFns...)
}

func TestFetchStatusesNoResources(t *testing.T) {
	is := is.New(t)

	client := &mockClient{
		fn: func(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error) {
			resources := []types.StackResource{}
			out := &cloudformation.DescribeStackResourcesOutput{
				StackResources: resources,
			}
			return out, nil
		},
	}
	s, err := fetchResourceStatuses(context.Background(), "", client)
	is.NoErr(err)
	is.Equal(s, []stackResource{})
}

func TestFetchOk(t *testing.T) {
	is := is.New(t)

	client := &mockClient{
		fn: func(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error) {
			resources := []types.StackResource{
				{
					LogicalResourceId: aws.String("Resource"),
					ResourceStatus:    types.ResourceStatusCreateComplete,
				},
			}
			out := &cloudformation.DescribeStackResourcesOutput{
				StackResources: resources,
			}
			return out, nil
		},
	}
	s, err := fetchResourceStatuses(context.Background(), "", client)
	is.NoErr(err)
	is.Equal(s, []stackResource{
		{
			resource: "Resource",
			status:   types.ResourceStatusCreateComplete,
		},
	})
}
