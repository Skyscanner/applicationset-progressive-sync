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

	. "github.com/onsi/gomega"
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

func TestReconcileStage(t *testing.T) {
	g := NewWithT(t)
	namespace := "progressivesync-test-" + randStringNumber(5)
	err := createNamespace(namespace)
	g.Expect(err).NotTo(HaveOccurred(), "unable to create namespace")
	defer func() {
		g.Expect(deleteNamespace(namespace)).To(Succeed())
	}()
}
