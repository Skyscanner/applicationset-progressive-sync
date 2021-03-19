package controllers

import (
	"context"
	"errors"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

type MockArgoCDAppClientSyncOK struct{}

func (c *MockArgoCDAppClientSyncOK) Sync(ctx context.Context, in *applicationpkg.ApplicationSyncRequest, opts ...grpc.CallOption) (*argov1alpha1.Application, error) {
	return &argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: *in.Name,
		},
	}, nil
}

type MockArgoCDAppClientSyncNotOK struct{}

func (c *MockArgoCDAppClientSyncNotOK) Sync(ctx context.Context, in *applicationpkg.ApplicationSyncRequest, opts ...grpc.CallOption) (*argov1alpha1.Application, error) {
	return nil, errors.New("rpc error: code = FailedPrecondition desc = authentication required")
}

func TestSync(t *testing.T) {
	r := ProgressiveRolloutReconciler{
		ArgoCDAppClient: &MockArgoCDAppClientSyncOK{},
	}

	testAppName := "foo-bar"

	application, error := r.syncApp(testAppName)

	g := NewWithT(t)
	g.Expect(error).To(BeNil())
	g.Expect(application.Name).To(Equal(testAppName))
}

func TestSyncErr(t *testing.T) {
	r := ProgressiveRolloutReconciler{
		ArgoCDAppClient: &MockArgoCDAppClientSyncNotOK{},
	}

	testAppName := "foo-bar"

	application, error := r.syncApp(testAppName)

	g := NewWithT(t)
	g.Expect(application).To(BeNil())
	g.Expect(error).ToNot(BeNil())
}
