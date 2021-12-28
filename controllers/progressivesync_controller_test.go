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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// func TestRequestsForApplicationChange(t *testing.T) {
// 	g := NewWithT(t)
// 	namespace := "progressivesync-test-" + randStringNumber(5)
// 	err := createNamespace(namespace)
// 	g.Expect(err).NotTo(HaveOccurred(), "unable to create namespace")
// 	defer func() {
// 		g.Expect(deleteNamespace(namespace)).To(Succeed())
// 	}()

// 	appSet := "owner-appset"
// 	_, err = createProgressiveSync("owner-ps", namespace, appSet)
// 	g.Expect(err).NotTo(HaveOccurred(), "unable to create progressivesync")

// 	t.Run("forward events for owned applications", func(t *testing.T) {
// 		app, err := createApplication("owned-app", namespace, appSet)
// 		g.Expect(err).NotTo(HaveOccurred(), "unable to create applications")

// 		var requests []reconcile.Request
// 		g.Eventually(func() int {
// 			requests = reconciler.requestsForApplicationChange(&app)
// 			return len(requests)
// 		}).Should(Equal(1))
// 	})

// 	t.Run("filter out events for non-owned applications", func(t *testing.T) {
// 		app, err := createApplication("non-owned-app", namespace, "wrong-owner")
// 		g.Expect(err).NotTo(HaveOccurred(), "unable to create applications")

// 		var requests []reconcile.Request
// 		g.Eventually(func() int {
// 			requests = reconciler.requestsForApplicationChange(&app)
// 			return len(requests)
// 		}).Should(Equal(0))
// 	})
// }

func TestReconcile(t *testing.T) {
	g := NewWithT(t)
	namespace := "progressivesync-test-" + randStringNumber(5)
	err := createNamespace(namespace)
	g.Expect(err).NotTo(HaveOccurred(), "unable to create namespace")
	defer func() {
		g.Expect(deleteNamespace(namespace)).To(Succeed())
		mockedClient.Reset()
	}()

	// Create an ApplicationSet which generated the Applications.
	// This is going to be
	appSet := "test-reconcile-appset"
	_, err = createApplicationSet(appSet, namespace)
	g.Expect(err).NotTo(HaveOccurred())

	t.Run("multi stage happy path", func(t *testing.T) {
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
		_, err = createSecrets(secrets, namespace)
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
		ps := newProgressiveSync("multi-stage-ps", namespace, appSet)
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

		// Ensure the controller starts the reconciliation
		var resultPs syncv1alpha1.ProgressiveSync
		g.Eventually(func() bool {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &resultPs)
			stage := resultPs.Status.LastSyncedStage == "one cluster as canary in eu-west-1"
			status := resultPs.Status.LastSyncedStageStatus == syncv1alpha1.StageStatusProgressing
			return stage && status
		}).Should(BeTrue())

		// Ensure the controller synced the correct Application
		g.Expect(mockedClient.GetSyncedApps()).Should(ContainElement("myservice-account1-eu-west-1a-1"))

		// Set myservice-account1-eu-west-1a-1 as synced
		err = setApplicationSyncStatus("myservice-account1-eu-west-1a-1", namespace, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		// Ensure the controller moves to the second stage once the first is completed
		g.Eventually(func() bool {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &resultPs)
			stage := resultPs.Status.LastSyncedStage == "one cluster as canary in every other region"
			status := resultPs.Status.LastSyncedStageStatus == syncv1alpha1.StageStatusProgressing
			return stage && status
		}).Should(BeTrue())

		//Ensure the controller synced the correct Applications
		g.Expect(mockedClient.GetSyncedApps()).Should(ContainElements([]string{
			"myservice-account2-eu-central-1a-1",
			"myservice-account2-eu-central-1b-1",
			"myservice-account3-ap-northeast-1a-1",
		}))

		// Set the second stage Applications as synced
		err = setApplicationSyncStatus("myservice-account2-eu-central-1a-1", namespace, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())
		err = setApplicationSyncStatus("myservice-account2-eu-central-1b-1", namespace, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())
		err = setApplicationSyncStatus("myservice-account3-ap-northeast-1a-1", namespace, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		// Ensure the controller moves to the third stage once the second stage is completed
		g.Eventually(func() bool {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &resultPs)
			stage := resultPs.Status.LastSyncedStage == "remaining clusters"
			status := resultPs.Status.LastSyncedStageStatus == syncv1alpha1.StageStatusProgressing
			return stage && status
		}).Should(BeTrue())

		// Ensure the controller synced only the first 2 out of 4 Applications
		g.Expect(mockedClient.GetSyncedApps()).Should(ContainElements([]string{
			"myservice-account1-eu-west-1b-1",
			"myservice-account3-ap-northeast-1b-1",
		}))

		// Set the first 2 Applications as synced
		err = setApplicationSyncStatus("myservice-account1-eu-west-1b-1", namespace, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())
		err = setApplicationSyncStatus("myservice-account3-ap-northeast-1b-1", namespace, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		// Ensure the controller synced the remaining Applications
		// Using envtually to give time to the controller to pick up the events
		g.Eventually(func() []string {
			return mockedClient.GetSyncedApps()
		}).Should(ContainElements([]string{
			"myservice-account4-ap-southeast-1a-1",
			"myservice-account4-ap-southeast-1b-1",
		}))

		// Set the remaining Applications as synced
		err = setApplicationSyncStatus("myservice-account4-ap-southeast-1a-1", namespace, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())
		err = setApplicationSyncStatus("myservice-account4-ap-southeast-1b-1", namespace, argov1alpha1.SyncStatusCodeSynced)
		g.Expect(err).NotTo(HaveOccurred())

		// Ensure the reconcile loop is completed
		g.Eventually(func() bool {
			_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &resultPs)
			stage := resultPs.Status.LastSyncedStage == "remaining clusters"
			status := resultPs.Status.LastSyncedStageStatus == syncv1alpha1.StageStatusCompleted
			ready := apimeta.IsStatusConditionTrue(resultPs.Status.Conditions, meta.ReadyCondition)
			return stage && status && ready
		}).Should(BeTrue())

		// Ensure the state map is correct
		stateMap, err := reconciler.ReadStateMap(ctx, getStateMapNamespacedName(ps))
		g.Expect(stateMap).NotTo(BeNil())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(stateMap.Apps["myservice-account1-eu-west-1a-1"].SyncedAtStage).To(Equal("one cluster as canary in eu-west-1"))
		g.Expect(stateMap.Apps["myservice-account1-eu-west-1b-1"].SyncedAtStage).To(Equal("remaining clusters"))
		g.Expect(stateMap.Apps["myservice-account2-eu-central-1a-1"].SyncedAtStage).To(Equal("one cluster as canary in every other region"))
		g.Expect(stateMap.Apps["myservice-account2-eu-central-1b-1"].SyncedAtStage).To(Equal("one cluster as canary in every other region"))
		g.Expect(stateMap.Apps["myservice-account3-ap-northeast-1a-1"].SyncedAtStage).To(Equal("one cluster as canary in every other region"))
		g.Expect(stateMap.Apps["myservice-account3-ap-northeast-1b-1"].SyncedAtStage).To(Equal("remaining clusters"))
		g.Expect(stateMap.Apps["myservice-account4-ap-southeast-1a-1"].SyncedAtStage).To(Equal("remaining clusters"))
		g.Expect(stateMap.Apps["myservice-account4-ap-southeast-1b-1"].SyncedAtStage).To(Equal("remaining clusters"))
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
