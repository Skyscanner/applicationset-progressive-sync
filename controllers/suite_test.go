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
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
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
	"github.com/argoproj/gitops-engine/pkg/health"
	//+kubebuilder:scaffold:imports
)

var (
	cancel       context.CancelFunc
	ctx          context.Context
	k8sClient    client.Client
	reconciler   *ProgressiveSyncReconciler
	testEnv      *envtest.Environment
	mockedClient mocks.MockArgoCDAppClientCalledWith
)

func init() {
	rand.Seed(time.Now().UnixNano())

	SetDefaultEventuallyTimeout(5 * time.Second)
	SetDefaultEventuallyPollingInterval(1 * time.Second)

	utilruntime.Must(syncv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(applicationset.AddToScheme(scheme.Scheme))
	utilruntime.Must(argov1alpha1.AddToScheme(scheme.Scheme))
}

func TestMain(m *testing.M) {
	var err error
	ctx, cancel = context.WithCancel(context.TODO())

	log.SetLogger(zap.New(zap.WriteTo(os.Stderr), zap.UseDevMode(true)))

	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "hack"),
		},
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(fmt.Sprintf("unabled to start envtest: %v", err))
	}

	// Uncached client
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
		ArgoCDAppClient: &mockedClient,
	}

	err = reconciler.SetupWithManager(k8sManager)
	if err != nil {
		panic(fmt.Sprintf("unabled to create reconciler: %v", err))
	}

	// Start the manager
	go func() {
		fmt.Println("starting the manager")
		if err := k8sManager.Start(ctx); err != nil {
			panic(fmt.Sprintf("unabled to start k8sManager: %v", err))
		}
	}()
	<-k8sManager.Elected()

	// Run the tests
	code := m.Run()

	// Stop the manager and the test environment
	cancel()
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("unable to stop the test environment: %v", err))
	}

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

func newProgressiveSync(name, namespace, appSet string) syncv1alpha1.ProgressiveSync {
	return syncv1alpha1.ProgressiveSync{
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
}

func newStage(name string, maxTargets, maxParallel int64, selector metav1.LabelSelector) syncv1alpha1.Stage {
	return syncv1alpha1.Stage{
		Name:        name,
		MaxParallel: maxParallel,
		MaxTargets:  maxTargets,
		Targets: syncv1alpha1.Targets{
			Clusters: syncv1alpha1.Clusters{
				Selector: selector,
			},
		},
	}
}

func createApplicationSet(name, namespace string) (applicationset.ApplicationSet, error) {
	appSet := applicationset.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: applicationset.ApplicationSetSpec{
			Generators: []applicationset.ApplicationSetGenerator{},
		},
	}
	return appSet, k8sClient.Create(ctx, &appSet)
}

// createApplication creates an Application targeting a cluster.
// The name MUST be in the format app_name-account_name-az_name-number.
func createApplication(name, namespace, appSet string) (argov1alpha1.Application, error) {
	cluster := strings.Join(strings.Split(name, "-")[1:], "-")
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
		Spec: argov1alpha1.ApplicationSpec{
			Destination: argov1alpha1.ApplicationDestination{
				Server:    fmt.Sprintf("https://%s.kubernetes.io", cluster),
				Namespace: namespace,
				Name:      cluster,
			}},
		Status: argov1alpha1.ApplicationStatus{
			Sync: argov1alpha1.SyncStatus{
				Status: argov1alpha1.SyncStatusCodeOutOfSync,
			},
			Health: argov1alpha1.HealthStatus{
				Status: health.HealthStatusHealthy,
			},
		},
	}
	return app, k8sClient.Create(ctx, &app)
}

func createApplications(names []string, namespace, appSet string) ([]argov1alpha1.Application, error) {
	var apps []argov1alpha1.Application
	for _, name := range names {
		app, err := createApplication(name, namespace, appSet)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func setApplicationSyncStatus(name, namespace string, status argov1alpha1.SyncStatusCode) error {
	var app argov1alpha1.Application

	if err := k8sClient.Get(ctx,
		types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
		&app,
	); err != nil {
		return err
	}

	app.Status.Sync.Status = status
	if err := k8sClient.Update(ctx, &app); err != nil {
		return err
	}
	return nil
}

// createSecret creates a secret with labels.
// The name MUST be in the format account_name-az_name-number,
// for example account1-eu-west-1a-1
func createSecret(name, namespace string) (corev1.Secret, error) {
	az := strings.Join(strings.Split(name, "-")[1:len(strings.Split(name, "-"))-1], "-")
	region := az[:len(az)-1]

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				consts.ArgoCDSecretTypeLabel: consts.ArgoCDSecretTypeCluster,
				"region":                     region,
				"az":                         az,
				"cluster":                    name,
			}},
		Data: map[string][]byte{
			"server": []byte(fmt.Sprintf("https://%s.kubernetes.io", name)),
		},
	}
	return secret, k8sClient.Create(ctx, &secret)
}

func createSecrets(names []string, namespace string) ([]corev1.Secret, error) {
	var secrets []corev1.Secret
	for _, name := range names {
		secret, err := createSecret(name, namespace)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, secret)
	}
	return secrets, nil
}
