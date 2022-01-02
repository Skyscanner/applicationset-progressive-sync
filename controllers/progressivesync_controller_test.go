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
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestRequestsForApplicationChange(t *testing.T) {
	g := NewWithT(t)
	ns, err := createNamespace("progressivesync-test-" + randStringNumber(5))
	g.Expect(err).NotTo(HaveOccurred(), "unable to create namespace")
	defer func() {
		g.Expect(deleteNamespace(ns)).To(Succeed())
	}()

	appSet := "test-requests-app-appset"
	_, err = createApplicationSet(appSet, ns.Name)
	g.Expect(err).NotTo(HaveOccurred())

	ps := newProgressiveSync("owner-ps", ns.Name, appSet)
	g.Expect(k8sClient.Create(ctx, &ps)).To(Succeed())

	t.Run("filters out events for non-owned applications", func(t *testing.T) {
		app, err := createApplication("non-owned-app", ns.Name, "wrong-owner")
		g.Expect(err).NotTo(HaveOccurred(), "unable to create applications")

		var requests []reconcile.Request
		g.Eventually(func() int {
			requests = reconciler.requestsForApplicationChange(&app)
			return len(requests)
		}).Should(Equal(0))

		assertHaveCondition(g, ps, meta.ReadyCondition)
	})

}

func TestReconcile(t *testing.T) {
	g := NewWithT(t)
	ns, err := createNamespace("progressivesync-test-" + randStringNumber(5))
	g.Expect(err).NotTo(HaveOccurred(), "unable to create namespace")
	defer func() {
		g.Expect(deleteNamespace(ns)).To(Succeed())
	}()

	t.Run("multi stage", func(t *testing.T) {
		defer mockedClient.Reset()

		// Create an ApplicationSet which generated the Applications.
		appSet := "test-reconcile-appset"
		_, err = createApplicationSet(appSet, ns.Name)
		g.Expect(err).NotTo(HaveOccurred())

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
		_, err = createSecrets(secrets, ns.Name)
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
		_, err = createApplications(apps, ns.Name, appSet)
		g.Expect(err).NotTo(HaveOccurred())

		// Create a multi-stage progressive sync
		ps := newProgressiveSync("multi-stage-ps", ns.Name, appSet)
		ps.Spec.Stages = []syncv1alpha1.Stage{
			newStage("one cluster as canary in eu-west-1", 1, 1, metav1.LabelSelector{
				MatchLabels: map[string]string{
					"region": "eu-west-1",
				}}),
			newStage("one cluster as canary in every other region", 3, 3, metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "region",
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"eu-west-1"},
				}}}),
			newStage("remaining clusters", 4, 2, metav1.LabelSelector{}),
		}
		g.Expect(k8sClient.Create(ctx, &ps)).To(Succeed())

		// Ensure the controller started the reconciliation
		assertHaveStageStatus(g, ps, "one cluster as canary in eu-west-1", syncv1alpha1.StageStatusProgressing)

		// Ensure the controller synced the correct Application
		assertHaveSyncedApp(g, "myservice-account1-eu-west-1a-1")
		assertHaveNotSyncedApp(g, "myservice-account1-eu-west-1b-1")
		assertHaveNotSyncedApp(g, "myservice-account2-eu-central-1a-1")
		assertHaveNotSyncedApp(g, "myservice-account2-eu-central-1b-1")
		assertHaveNotSyncedApp(g, "myservice-account3-ap-northeast-1a-1")
		assertHaveNotSyncedApp(g, "myservice-account3-ap-northeast-1b-1")
		assertHaveNotSyncedApp(g, "myservice-account4-ap-southeast-1a-1")
		assertHaveNotSyncedApp(g, "myservice-account4-ap-southeast-1b-1")

		// Set myservice-account1-eu-west-1a-1 as synced
		err = setApplicationSyncStatus("myservice-account1-eu-west-1a-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		// Ensure the controller moves to the second stage once the first is completed
		assertHaveStageStatus(g, ps, "one cluster as canary in every other region", syncv1alpha1.StageStatusProgressing)

		//Ensure the controller synced the correct Applications
		assertHaveSyncedApp(g, "myservice-account2-eu-central-1a-1")
		assertHaveSyncedApp(g, "myservice-account2-eu-central-1b-1")
		assertHaveSyncedApp(g, "myservice-account3-ap-northeast-1a-1")
		assertHaveNotSyncedApp(g, "myservice-account1-eu-west-1b-1")
		assertHaveNotSyncedApp(g, "myservice-account3-ap-northeast-1b-1")
		assertHaveNotSyncedApp(g, "myservice-account4-ap-southeast-1a-1")
		assertHaveNotSyncedApp(g, "myservice-account4-ap-southeast-1b-1")

		// Set the second stage Applications as synced
		err = setApplicationSyncStatus("myservice-account2-eu-central-1a-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())
		err = setApplicationSyncStatus("myservice-account2-eu-central-1b-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())
		err = setApplicationSyncStatus("myservice-account3-ap-northeast-1a-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		// Ensure the controller moved to the third stage once the second stage is completed
		assertHaveStageStatus(g, ps, "remaining clusters", syncv1alpha1.StageStatusProgressing)

		// Ensure the controller synced only the first 2 out of 4 Applications
		assertHaveSyncedApp(g, "myservice-account1-eu-west-1b-1")
		assertHaveSyncedApp(g, "myservice-account3-ap-northeast-1b-1")
		assertHaveNotSyncedApp(g, "myservice-account4-ap-southeast-1a-1")
		assertHaveNotSyncedApp(g, "myservice-account4-ap-southeast-1b-1")

		// Set the first 2 Applications as synced
		err = setApplicationSyncStatus("myservice-account1-eu-west-1b-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())
		err = setApplicationSyncStatus("myservice-account3-ap-northeast-1b-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		// Ensure the controller synced the remaining Applications
		// Using envtually to give time to the controller to pick up the events
		assertHaveSyncedApp(g, "myservice-account4-ap-southeast-1a-1")
		assertHaveSyncedApp(g, "myservice-account4-ap-southeast-1b-1")

		// Set the remaining Applications as synced
		err = setApplicationSyncStatus("myservice-account4-ap-southeast-1a-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())
		err = setApplicationSyncStatus("myservice-account4-ap-southeast-1b-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		// Ensure the reconcile loop is completed
		assertHaveStageStatus(g, ps, "remaining clusters", syncv1alpha1.StageStatusCompleted)
		assertHaveCondition(g, ps, meta.ReadyCondition)

		// Ensure the state map is correct
		assertHaveSyncedAtStage(g, ps, "myservice-account1-eu-west-1a-1", "one cluster as canary in eu-west-1")
		assertHaveSyncedAtStage(g, ps, "myservice-account1-eu-west-1b-1", "remaining clusters")
		assertHaveSyncedAtStage(g, ps, "myservice-account2-eu-central-1a-1", "one cluster as canary in every other region")
		assertHaveSyncedAtStage(g, ps, "myservice-account2-eu-central-1b-1", "one cluster as canary in every other region")
		assertHaveSyncedAtStage(g, ps, "myservice-account3-ap-northeast-1a-1", "one cluster as canary in every other region")
		assertHaveSyncedAtStage(g, ps, "myservice-account3-ap-northeast-1b-1", "remaining clusters")
		assertHaveSyncedAtStage(g, ps, "myservice-account4-ap-southeast-1a-1", "remaining clusters")
		assertHaveSyncedAtStage(g, ps, "myservice-account4-ap-southeast-1b-1", "remaining clusters")

		// Ownership test for deletion
		// See https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		boolTrue := true
		expectedOwnerReference := metav1.OwnerReference{
			Kind:               "ProgressiveSync",
			APIVersion:         "argoproj.skyscanner.net/v1alpha1",
			UID:                ps.GetUID(),
			Name:               ps.Name,
			Controller:         &boolTrue,
			BlockOwnerDeletion: &boolTrue,
		}
		g.Eventually(func() []metav1.OwnerReference {
			key := getStateMapNamespacedName(ps)
			cm := corev1.ConfigMap{}
			_ = k8sClient.Get(ctx, key, &cm)
			return cm.ObjectMeta.OwnerReferences
		}).Should(ContainElement(expectedOwnerReference))

	})

	t.Run("complete when no OutOfSync apps", func(t *testing.T) {
		defer mockedClient.Reset()

		// Create an ApplicationSet which generated the Applications.
		appSet := "no-out-of-sync"
		_, err = createApplicationSet(appSet, ns.Name)
		g.Expect(err).NotTo(HaveOccurred())

		// Create clusters
		secrets := []string{
			"account5-eu-west-1a-1",
			"account5-eu-west-1b-1",
		}
		_, err = createSecrets(secrets, ns.Name)
		g.Expect(err).NotTo(HaveOccurred())

		// Create Applications targeting the clusters
		apps := []string{
			"myservice-account5-eu-west-1a-1",
			"myservice-account5-eu-west-1b-1",
		}
		_, err = createApplications(apps, ns.Name, appSet)
		g.Expect(err).NotTo(HaveOccurred())

		// Set the Applications as synced
		err = setApplicationSyncStatus("myservice-account5-eu-west-1a-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())
		err = setApplicationSyncStatus("myservice-account5-eu-west-1b-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		ps := newProgressiveSync("no-out-of-sync", ns.Name, appSet)
		ps.Spec.Stages = []syncv1alpha1.Stage{
			newStage("stage one", 1, 1, metav1.LabelSelector{}),
		}
		g.Expect(k8sClient.Create(ctx, &ps)).To(Succeed())

		// Ensure the reconciliation is completed
		assertHaveStageStatus(g, ps, "stage one", syncv1alpha1.StageStatusCompleted)
		assertHaveCondition(g, ps, meta.ReadyCondition)

		// Ensure the controller didn't sync any Application
		assertHaveNotSyncedApp(g, "myservice-account5-eu-west-1a-1")
		assertHaveNotSyncedApp(g, "myservice-account5-eu-west-1b-1")

		// Ensure we adopted the synced Application
		assertHaveSyncedAtStage(g, ps, "myservice-account5-eu-west-1a-1", "stage one")
	})

	t.Run("complete when a stage is targeting zero clusters", func(t *testing.T) {
		defer mockedClient.Reset()

		// Create an ApplicationSet which generated the Applications.
		appSet := "zero-clusters"
		_, err = createApplicationSet(appSet, ns.Name)
		g.Expect(err).NotTo(HaveOccurred())

		// Create a cluster
		_, err = createSecret("account6-eu-west-1a-1", ns.Name)
		g.Expect(err).NotTo(HaveOccurred())

		// Create an Application targeting the cluster
		_, err = createApplication("myservice-account6-eu-west-1a-1", ns.Name, appSet)
		g.Expect(err).NotTo(HaveOccurred())

		ps := newProgressiveSync("zero-clusters", ns.Name, appSet)
		ps.Spec.Stages = []syncv1alpha1.Stage{
			newStage("stage one cluster", 1, 1, metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster": "account6-eu-west-1a-1",
				}}),
			newStage("stage other regions", 3, 3, metav1.LabelSelector{
				MatchLabels: map[string]string{
					"region": "eu-west-1",
				}}),
		}
		g.Expect(k8sClient.Create(ctx, &ps)).To(Succeed())

		// Set the Application as synced
		err = setApplicationSyncStatus("myservice-account6-eu-west-1a-1", ns.Name, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		// Ensure the reconciliation is completed
		assertHaveStageStatus(g, ps, "stage other regions", syncv1alpha1.StageStatusCompleted)
		assertHaveCondition(g, ps, meta.ReadyCondition)
	})
}
