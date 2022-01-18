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

package v1alpha1

import (
	"testing"

	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOwns(t *testing.T) {
	testCases := []struct {
		ownerReferences []metav1.OwnerReference
		expected        bool
	}{{
		ownerReferences: []metav1.OwnerReference{{
			APIVersion: "fakeAPIVersion",
			Kind:       "fakeKind",
			Name:       "fakeName",
		}, {
			APIVersion: consts.AppSetAPIVersion,
			Kind:       consts.AppSetKind,
			Name:       "owner-appset",
		}},
		expected: true,
	}, {
		ownerReferences: []metav1.OwnerReference{{
			APIVersion: "fakeAPIVersion",
			Kind:       "fakeKind",
			Name:       "fakeName",
		}},
		expected: false,
	}}

	for _, tt := range testCases {
		g := NewWithT(t)
		ps := ProgressiveSync{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: ProgressiveSyncSpec{
				AppSetRef: meta.LocalObjectReference{
					Name: "owner-appset",
				},
			},
		}
		got := ps.Owns(tt.ownerReferences)
		g.Expect(got).To(Equal(tt.expected))
	}
}
