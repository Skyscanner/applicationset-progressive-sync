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
	"fmt"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// StateData holds a state for the stage reconciliation
type StateData struct {
	AppSetHash string              `yaml:"appSetHash"`
	Apps       map[string]AppState `yaml:"apps"`
}

// AppState holds the state for an application
type AppState struct {
	SyncedAtStage string `yaml:"syncedAtStage"`
}

// CreateStateMap creates the state configmap
func (r *ProgressiveSyncReconciler) CreateStateMap(ctx context.Context, ps syncv1alpha1.ProgressiveSync) error {
	key := getStateMapNamespacedName(ps)
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}

	// Check if the configmap already exists
	if err := r.Get(ctx, key, &cm); err != nil {
		if !errors.IsNotFound(err) {
			return err
		} else {
			// Set the ownership and create the configmap
			if refErr := controllerutil.SetControllerReference(&ps, &cm, r.Scheme); refErr != nil {
				return refErr
			}
			if cErr := r.Create(ctx, &cm); cErr != nil {
				return cErr
			}
		}
	}

	return nil
}

// DeleteStateMap deletes the state configmap
func (r *ProgressiveSyncReconciler) DeleteStateMap(ctx context.Context, ps syncv1alpha1.ProgressiveSync) error {
	key := getStateMapNamespacedName(ps)
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	if err := r.Delete(ctx, &cm, &client.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// ReadStateMap reads the state configmap and returns the state data structure
func (r *ProgressiveSyncReconciler) ReadStateMap(ctx context.Context, ps syncv1alpha1.ProgressiveSync) (StateData, error) {
	var stateData StateData
	var cm corev1.ConfigMap

	key := getStateMapNamespacedName(ps)

	if err := r.Get(ctx, key, &cm); err != nil {
		return stateData, err
	}

	if err := yaml.Unmarshal([]byte(cm.Data["appSetHash"]), &stateData.AppSetHash); err != nil {
		return stateData, err
	}

	if err := yaml.Unmarshal([]byte(cm.Data["apps"]), &stateData.Apps); err != nil {
		return stateData, err
	}

	// Make sure we initiliaze the map before adding any element to it
	if stateData.Apps == nil {
		stateData.Apps = make(map[string]AppState)
	}

	return stateData, nil
}

// UpdateStateMap writes the state data structure into the state configmap
func (r *ProgressiveSyncReconciler) UpdateStateMap(ctx context.Context, ps syncv1alpha1.ProgressiveSync, state StateData) error {

	appSetHash, err := yaml.Marshal(state.AppSetHash)
	if err != nil {
		return err
	}

	apps, err := yaml.Marshal(state.Apps)
	if err != nil {
		return err
	}

	key := getStateMapNamespacedName(ps)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: map[string]string{
			"appSetHash": string(appSetHash),
			"apps":       string(apps),
		},
	}

	if err := r.Update(ctx, &cm, &client.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

// getStateMapNamespacedName returns the state namespaced name constructed from the ProgressiveSync object
func getStateMapNamespacedName(ps syncv1alpha1.ProgressiveSync) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("progressive-sync-state-%s", ps.Name),
		Namespace: ps.Namespace,
	}
}
