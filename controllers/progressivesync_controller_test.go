/*
Copyright 2021 Skyscanner Limited.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"testing"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestRequestsForApplicationChange(t *testing.T) {
	g := NewWithT(t)
	namespace := "progressivesync-test-" + randStringNumber(5)
	err := createNamespace(namespace)
	g.Expect(err).NotTo(HaveOccurred(), "unable to create namespace")
	defer func() {
		g.Expect(deleteNamespace(namespace)).To(Succeed())
	}()

	appSet := "owner-appset"
	_, err = createProgressiveSync("owner-ps", namespace, appSet)
	g.Expect(err).NotTo(HaveOccurred(), "unable to create progressivesync")

	t.Run("forward events for owned applications", func(t *testing.T) {
		app, err := createApplication("owned-app", namespace, appSet)
		g.Expect(err).NotTo(HaveOccurred(), "unable to create applications")

		var requests []reconcile.Request
		g.Eventually(func() int {
			requests = reconciler.requestsForApplicationChange(&app)
			return len(requests)
		}).Should(Equal(1))
	})

	t.Run("filter out events for non-owned applications", func(t *testing.T) {
		app, err := createApplication("non-owned-app", namespace, "wrong-owner")
		g.Expect(err).NotTo(HaveOccurred(), "unable to create applications")

		var requests []reconcile.Request
		g.Eventually(func() int {
			requests = reconciler.requestsForApplicationChange(&app)
			return len(requests)
		}).Should(Equal(0))
	})
}

func TestReconcile(t *testing.T) {
	g := NewWithT(t)
	namespace := "progressivesync-test-" + randStringNumber(5)
	err := createNamespace(namespace)
	g.Expect(err).NotTo(HaveOccurred(), "unable to create namespace")
	defer func() {
		g.Expect(deleteNamespace(namespace)).To(Succeed())
	}()

	appSet := "multi-stage-appset"

	t.Run("reconcile multi stage", func(t *testing.T) {
		// Create eight ArgoCD clusters across multiple regions
		secrets := []string{
			"account1-eu-west-1a-1",
			"account1-eu-west-1b-1",
			"account2-eu-central-1a-1",
			"account2-eu-central-1b-1",
			"account3-ap-northeast-1a-1",
			"account3-ap-northeast-1b-1",
			"account4-ap-southeast-1a-1",
			"account4-ap-southeast-1b-1",
		}
		_, err := createSecrets(secrets, namespace)
		g.Expect(err).NotTo(HaveOccurred())

		// Create eight Applications targeting the clusters
		apps := []string{
			"myservice-account1-eu-west-1a-1",
			"myservice-account1-eu-west-1b-1",
			"myservice-account2-eu-central-1a-1",
			"myservice-account2-eu-central-1b-1",
			"myservice-account3-ap-northeast-1a-1",
			"myservice-account3-ap-northeast-1b-1",
			"myservice-account4-ap-southeast-1a-1",
			"myservice-account4-ap-southeast-1b-1",
		}
		_, err = createApplications(apps, namespace, appSet)
		g.Expect(err).NotTo(HaveOccurred())

		// Create a multi-stage progressive sync
		ps, err := createProgressiveSync("multi-stage-ps", namespace, appSet)
		g.Expect(err).NotTo(HaveOccurred())

		ps.Spec.Stages = []syncv1alpha1.Stage{
			createStage("one cluster as canary in eu-west-1", 1, 1, metav1.LabelSelector{
				MatchLabels: map[string]string{
					"region": "eu-west-1",
				}}),
			createStage("one cluster as canary in every other region", 3, 3, metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "region",
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"eu-west-1"},
				}}}),
			createStage("remaining clusters", 4, 2, metav1.LabelSelector{}),
		}
	})
}

// func TestReconcileStage(t *testing.T) {
// 	g := NewWithT(t)
// 	namespace := "progressivesync-test-" + randStringNumber(5)
// 	err := createNamespace(namespace)
// 	g.Expect(err).NotTo(HaveOccurred(), "unable to create namespace")
// 	defer func() {
// 		g.Expect(deleteNamespace(namespace)).To(Succeed())
// 	}()

// 	// Create a ProgressiveSync object with its own state configmap
// 	appSet := "test-stage-appset"
// 	ps, err := createProgressiveSync("test-stage", namespace, appSet)
// 	g.Expect(err).NotTo(HaveOccurred())
// 	err = reconciler.createStateMap(ctx, ps, getStateMapNamespacedName(ps))
// 	g.Expect(err).NotTo(HaveOccurred())

// 	t.Run("stage completed when no OutOfSync apps", func(t *testing.T) {
// 		// Create an ArgoCD cluster
// 		_, err := createSecret("account1-eu-west-1a-1", namespace)
// 		g.Expect(err).NotTo(HaveOccurred())

// 		// Create a synced ArgoCD Application targeting the cluster
// 		name := "myservice-account1-eu-west-1a-1"
// 		_, err = createApplication(name, namespace, appSet)
// 		g.Expect(err).NotTo(HaveOccurred())

// 		// Set the Application as Synced so there are no OutOfSync apps
// 		err = setApplicationSyncStatus(name, namespace, argov1alpha1.SyncStatusCodeSynced)
// 		g.Expect(err).NotTo(HaveOccurred())

// 		// Add a stage to the ProgressiveSync targeting the cluster
// 		ps := ps.DeepCopy()
// 		stage := createStage("stage-one", 1, 1, metav1.LabelSelector{})
// 		ps.Spec.Stages = []syncv1alpha1.Stage{
// 			stage,
// 		}
// 		stageStatus, err := reconciler.reconcileStage(ctx, *ps, stage)
// 		g.Expect(err).NotTo(HaveOccurred())
// 		g.Expect(stageStatus).To(Equal(syncv1alpha1.StageStatusCompleted))
// 	})
// }
