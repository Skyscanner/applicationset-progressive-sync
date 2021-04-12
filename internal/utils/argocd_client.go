package utils

import (
	"context"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"google.golang.org/grpc"
)

type ArgoCDAppClient interface {
	Sync(ctx context.Context, in *applicationpkg.ApplicationSyncRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error)
}

// GetArgoCDAppClient builds an Argo CD application client with the provided configuration
func GetArgoCDAppClient(c Configuration) ArgoCDAppClient {
	acdClientOpts := argocdclient.ClientOptions{
		ServerAddr: c.ArgoCDServerAddr,
		Insecure:   c.ArgoCDInsecure,
		AuthToken:  c.ArgoCDAuthToken,
	}

	acdClient := argocdclient.NewClientOrDie(&acdClientOpts)
	_, acdAppClient := acdClient.NewApplicationClientOrDie()

	return acdAppClient
}
