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
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
	"github.com/Skyscanner/applicationset-progressive-sync/mocks"
	applicationset "github.com/argoproj-labs/applicationset/api/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var (
	cancel     context.CancelFunc
	ctx        context.Context
	k8sClient  client.Client
	reconciler *ProgressiveSyncReconciler
	testEnv    *envtest.Environment
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestMain(m *testing.M) {
	var err error
	ctx, cancel = context.WithCancel(context.TODO())

	log.SetLogger(zap.New(zap.WriteTo(os.Stderr), zap.UseDevMode(true)))

	utilruntime.Must(syncv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(applicationset.AddToScheme(scheme.Scheme))
	utilruntime.Must(argov1alpha1.AddToScheme(scheme.Scheme))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "hack"),
		},
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(fmt.Sprintf("unabled to start envtest: %v", err))
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(fmt.Sprintf("unabled to create k8sClient: %v", err))
	}

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		panic(fmt.Sprintf("unabled to create k8sManager: %v", err))
	}

	reconciler = &ProgressiveSyncReconciler{
		Client:          k8sManager.GetClient(),
		Scheme:          k8sManager.GetScheme(),
		ArgoCDAppClient: &mocks.MockArgoCDAppClientCalledWith{},
	}

	err = reconciler.SetupWithManager(k8sManager)
	if err != nil {
		panic(fmt.Sprintf("unabled to create reconciler: %v", err))
	}

	go func() {
		fmt.Println("starting the manager")
		if err := k8sManager.Start(ctx); err != nil {
			panic(fmt.Sprintf("unabled to start k8sManager: %v", err))
		}
	}()

	code := m.Run()

	os.Exit(code)
}

var numbers = []rune("1234567890")

func randStringNumber(n int) string {
	s := make([]rune, n)
	for i := range s {
		s[i] = numbers[rand.Intn(len(numbers))]
	}
	return string(s)
}

func createNamespace(name string) error {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	return k8sClient.Create(ctx, &namespace)

}

func deleteNamespace(name string) error {
	var err error
	var ns corev1.Namespace
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name: name,
	}, &ns)
	if err != nil {
		return err
	}
	err = k8sClient.Delete(ctx, &ns)
	if err != nil {
		return err
	}
	return nil
}

func createProgressiveSync(name, namespace, appSet string) (syncv1alpha1.ProgressiveSync, error) {
	ps := syncv1alpha1.ProgressiveSync{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: syncv1alpha1.ProgressiveSyncSpec{
			AppSetRef: meta.LocalObjectReference{
				Name: appSet,
			},
		},
	}
	return ps, k8sClient.Create(ctx, &ps)
}

func createApplication(name, namespace, appSet string) (argov1alpha1.Application, error) {
	app := argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: consts.AppSetAPIVersion,
				Kind:       consts.AppSetKind,
				Name:       appSet,
				UID:        uuid.NewUUID(),
			}},
		},
		Spec: argov1alpha1.ApplicationSpec{},
	}
	return app, k8sClient.Create(ctx, &app)
}
