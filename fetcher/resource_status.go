package fetcher

import "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

type stackResource struct {
	resource string
	status   types.ResourceStatus
}

type ResourceStatuses map[string]types.ResourceStatus

func NewResourceStatuses() *ResourceStatuses {
	res := make(ResourceStatuses)
	return &res
}
