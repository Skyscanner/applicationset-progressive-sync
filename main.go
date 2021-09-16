/**
 * Copyright 2021 Skyscanner Limited.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/utils"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/controllers"
	applicationset "github.com/argoproj-labs/applicationset/api/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = syncv1alpha1.AddToScheme(scheme)
	_ = argov1alpha1.AddToScheme(scheme)
	_ = applicationset.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr, ctrlNamespace, argoNamespace string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&ctrlNamespace, "ctrl-namespace", "argocd", "The controller namespace")
	flag.StringVar(&argoNamespace, "argo-namespace", "argocd", "The namespace where ArgoCD and the ApplicationSet controller are deployed to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	namespaces := []string{ctrlNamespace}
	if ctrlNamespace != argoNamespace {
		namespaces = append(namespaces, argoNamespace)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		NewCache:           cache.MultiNamespacedCacheBuilder(namespaces),
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "b84175a0.argoproj.skyscanner.net",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	c, err := utils.ReadConfiguration()
	if err != nil {
		setupLog.Error(err, "unable to read configuration")
		os.Exit(1)
	}

	if err = (&controllers.ProgressiveSyncReconciler{
		Client:          mgr.GetClient(),
		Log:             ctrl.Log.WithName("controllers").WithName("ProgressiveSync"),
		Scheme:          mgr.GetScheme(),
		ArgoCDAppClient: utils.GetArgoCDAppClient(c),
		StateManager:    utils.NewProgressiveSyncManager(),
		ArgoNamespace:   argoNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProgressiveSync")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
