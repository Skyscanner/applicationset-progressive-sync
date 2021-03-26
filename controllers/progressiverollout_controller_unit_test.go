package controllers

import (
	"github.com/Skyscanner/argocd-progressive-rollout/mocks"
	. "github.com/onsi/gomega"
	"testing"
)

func TestSync(t *testing.T) {
	r := ProgressiveRolloutReconciler{
		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncOK{},
	}

	testAppName := "foo-bar"

	application, error := r.syncApp(testAppName)

	g := NewWithT(t)
	g.Expect(error).To(BeNil())
	g.Expect(application.Name).To(Equal(testAppName))
}

func TestSyncErr(t *testing.T) {
	r := ProgressiveRolloutReconciler{
		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncNotOK{},
	}

	testAppName := "foo-bar"

	application, error := r.syncApp(testAppName)

	g := NewWithT(t)
	g.Expect(application).To(BeNil())
	g.Expect(error).ToNot(BeNil())
}
