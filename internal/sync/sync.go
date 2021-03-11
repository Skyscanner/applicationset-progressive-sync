package sync

import (
	"context"
	"github.com/Skyscanner/argocd-progressive-rollout/internal/utils"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

// Sync sends a sync request to the target app
func Sync(app argov1alpha1.Application, ctx context.Context) (*argov1alpha1.Application, error) {
	serverAddr, err := utils.GetArgoServerAddr()
	if err != nil {
		return nil, err
	}
	authToken, err := utils.GetArgoAuthToken()
	if err != nil {
		return nil, err
	}

	clientOpts := argocdclient.ClientOptions{
		ServerAddr: serverAddr,
		Insecure:   true,
		AuthToken:  authToken,
	}

	acdClient := argocdclient.NewClientOrDie(&clientOpts)
	_, appIf := acdClient.NewApplicationClientOrDie()

	name := &app.Name

	syncReq := applicationpkg.ApplicationSyncRequest{
		Name: name,
	}

	result, err := appIf.Sync(ctx, &syncReq)
	if err != nil {
		return nil, err
	}

	return result, nil
}
