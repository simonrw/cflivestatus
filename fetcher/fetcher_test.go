package fetcher

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
	fns []handlerFunc
	i   int
}

func (m *mockClient) DescribeStackResources(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error) {
	if m.fns == nil {
		panic("missing function handlers")
	}
	if m.i >= len(m.fns) {
		panic("too few functions defined")
	}
	res, err := m.fns[m.i](ctx, params, optFns...)
	m.i++
	return res, err
}

func (m *mockClient) assertNumFunctionsCalled(t *testing.T) {
	if m.i != len(m.fns) {
		t.Fatalf("too few function calls compared to setup, found %d expected %d", m.i, len(m.fns))
	}
}

func TestFetchStatusesNoResources(t *testing.T) {
	is := is.New(t)

	client := &mockClient{}
	client.fns = append(client.fns, func(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error) {
		resources := []types.StackResource{}
		out := &cloudformation.DescribeStackResourcesOutput{
			StackResources: resources,
		}
		return out, nil
	})
	defer client.assertNumFunctionsCalled(t)

	fetcher := fetcher{client: client}

	res, err := fetcher.Fetch(context.Background())
	is.NoErr(err)
	is.Equal(res, []StackResource{})
}

func TestFetchOk(t *testing.T) {
	is := is.New(t)

	client := &mockClient{}
	client.fns = append(client.fns, func(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error) {
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
	})
	defer client.assertNumFunctionsCalled(t)

	fetcher := fetcher{client: client}
	res, err := fetcher.Fetch(context.Background())
	is.NoErr(err)
	is.Equal(res, []StackResource{
		{
			Resource: "Resource",
			Status:   types.ResourceStatusCreateComplete,
		},
	})
}

func TestFetchTwoUpdates(t *testing.T) {
	is := is.New(t)

	client := &mockClient{}
	client.fns = append(client.fns, func(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error) {
		resources := []types.StackResource{
			{
				LogicalResourceId: aws.String("Resource"),
				ResourceStatus:    types.ResourceStatusCreateInProgress,
			},
		}
		out := &cloudformation.DescribeStackResourcesOutput{
			StackResources: resources,
		}
		return out, nil
	})
	client.fns = append(client.fns, func(ctx context.Context, params *cloudformation.DescribeStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackResourcesOutput, error) {
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
	})
	defer client.assertNumFunctionsCalled(t)

	fetcher := fetcher{client: client}
	res, err := fetcher.Fetch(context.Background())
	is.NoErr(err)
	is.Equal(res, []StackResource{
		{
			Resource: "Resource",
			Status:   types.ResourceStatusCreateInProgress,
		},
	})

	res, err = fetcher.Fetch(context.Background())
	is.NoErr(err)
	is.Equal(res, []StackResource{
		{
			Resource: "Resource",
			Status:   types.ResourceStatusCreateComplete,
		},
	})
}
