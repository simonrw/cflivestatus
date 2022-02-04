package fetcher

import "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

type StackResource struct {
	Resource string
	Status   types.ResourceStatus
}
