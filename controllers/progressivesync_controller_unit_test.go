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
	"context"
	"testing"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
	"github.com/Skyscanner/applicationset-progressive-sync/mocks"
	applicationset "github.com/argoproj-labs/applicationset/api/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const namespace = "progressivesync-system"

func TestProgressiveSyncReconciler_reconcileStage(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(argov1alpha1.AddToScheme(scheme))
	utilruntime.Must(applicationset.AddToScheme(scheme))
	utilruntime.Must(syncv1alpha1.AddToScheme(scheme))

	appSetAPIGroup := consts.AppSetAPIGroup
	ctx := context.Background()

	ps := syncv1alpha1.ProgressiveSync{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ps",
			Namespace: namespace,
		},
		Spec: syncv1alpha1.ProgressiveSyncSpec{
			SourceRef: corev1.TypedLocalObjectReference{
				Name:     "appset-ps",
				Kind:     consts.AppSetKind,
				APIGroup: &appSetAPIGroup,
			},
			Stages: []syncv1alpha1.Stage{},
		},
	}

	tests := []struct {
		name            string
		resources       []runtime.Object
		stage           syncv1alpha1.Stage
		wantStageStatus syncv1alpha1.StageStatus
		wantStageErr    bool
	}{
		{
			name:      "Applications: outOfSync 3, syncedInCurrentStage 0, progressing 0, | Stage: maxTargets 2, maxParallel 2 | Expected: scheduled 2",
			resources: []runtime.Object{},
			stage: syncv1alpha1.Stage{
				Name:        "stage",
				MaxTargets:  2,
				MaxParallel: 2,
			},
			wantStageStatus: syncv1alpha1.StageStatus(syncv1alpha1.StageStatusProgressing),
			wantStageErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ps.Spec.Stages = append(ps.Spec.Stages, tt.stage)
			tt.resources = append(tt.resources, &ps)

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tt.resources...).
				Build()
			r := &ProgressiveSyncReconciler{
				Client:          client,
				Scheme:          scheme,
				ArgoCDAppClient: &mocks.ArgoCDAppClientStub{},
				ArgoNamespace:   namespace,
			}

			err := r.createStateMap(ctx, ps, getStateMapNamespacedName(ps))
			g.Expect(err).To(BeNil())

			gotStageStatus, gotStageErr := r.reconcileStage(ctx, ps, tt.stage)
			g.Expect(gotStageStatus).To(Equal(tt.wantStageStatus))
			if gotStageErr == nil {
				g.Expect(gotStageErr).To(BeNil())
			} else {
				g.Expect(gotStageErr.Error()).To(Equal(tt.wantStageErr))

			}
		})
	}

}
