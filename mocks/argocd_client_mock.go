package mocks

import (
	"context"
	"errors"
	"sync"

	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ArgoCDAppClientStub is a general-purpose stub for the Argo CD Application Client
type ArgoCDAppClientStub struct{}

func (*ArgoCDAppClientStub) Sync(ctx context.Context, in *applicationpkg.ApplicationSyncRequest, opts ...grpc.CallOption) (*argov1alpha1.Application, error) {
	return nil, nil
}

// MockArgoCDAppClientCalledWith mocks the Argo CD Application client and registers calls to Sync
type MockArgoCDAppClientCalledWith struct {
	appsSynced []string
	m          sync.Mutex
}

func (c *MockArgoCDAppClientCalledWith) Sync(ctx context.Context, in *applicationpkg.ApplicationSyncRequest, opts ...grpc.CallOption) (*argov1alpha1.Application, error) {
	c.m.Lock()
	defer c.m.Unlock()
	c.appsSynced = append(c.appsSynced, *in.Name)

	return nil, nil
}

func (c *MockArgoCDAppClientCalledWith) GetSyncedApps() []string {
	c.m.Lock()
	defer c.m.Unlock()

	return c.appsSynced
}

func (c *MockArgoCDAppClientCalledWith) Reset() {
	c.m.Lock()
	defer c.m.Unlock()
	c.appsSynced = []string{}
}

// MockArgoCDAppClientSyncOK mocks the Argo CD Application client with a successful invocation of Sync
type MockArgoCDAppClientSyncOK struct{}

func (c *MockArgoCDAppClientSyncOK) Sync(ctx context.Context, in *applicationpkg.ApplicationSyncRequest, opts ...grpc.CallOption) (*argov1alpha1.Application, error) {
	return &argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: *in.Name,
		},
	}, nil
}

// MockArgoCDAppClientSyncOK mocks the Argo CD Application client with a failed invocation of Sync
type MockArgoCDAppClientSyncNotOK struct{}

func (c *MockArgoCDAppClientSyncNotOK) Sync(ctx context.Context, in *applicationpkg.ApplicationSyncRequest, opts ...grpc.CallOption) (*argov1alpha1.Application, error) {
	return nil, errors.New("rpc error: code = FailedPrecondition desc = authentication required")
}
