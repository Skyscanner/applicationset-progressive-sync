module github.com/Skyscanner/argocd-progressive-rollout

go 1.15

require (
	github.com/argoproj/argo-cd v0.8.1-0.20210218100039-a4ee25b59d8d
	github.com/argoproj/gitops-engine v0.2.1-0.20210129183711-c5b7114c501f
	github.com/go-logr/logr v0.3.0
	github.com/kr/pretty v0.2.1 // indirect
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.11.0
	google.golang.org/grpc v1.33.1
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v11.0.1-0.20190816222228-6d55c1b1f1ca+incompatible
	sigs.k8s.io/controller-runtime v0.8.0
	sigs.k8s.io/controller-tools v0.2.5
)

replace k8s.io/api => k8s.io/api v0.20.1

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.1

replace k8s.io/apimachinery => k8s.io/apimachinery v0.21.0-alpha.0

replace k8s.io/apiserver => k8s.io/apiserver v0.20.1

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.1

replace k8s.io/client-go => k8s.io/client-go v0.20.1

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.20.1

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.20.1

replace k8s.io/code-generator => k8s.io/code-generator v0.20.5-rc.0

replace k8s.io/component-base => k8s.io/component-base v0.20.1

replace k8s.io/component-helpers => k8s.io/component-helpers v0.20.1

replace k8s.io/controller-manager => k8s.io/controller-manager v0.20.1

replace k8s.io/cri-api => k8s.io/cri-api v0.20.5-rc.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.20.1

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.20.1

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.20.1

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.20.1

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.20.1

replace k8s.io/kubectl => k8s.io/kubectl v0.20.1

replace k8s.io/kubelet => k8s.io/kubelet v0.20.1

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.20.1

replace k8s.io/metrics => k8s.io/metrics v0.20.1

replace k8s.io/mount-utils => k8s.io/mount-utils v0.20.5-rc.0

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.20.1

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.20.1

replace k8s.io/sample-controller => k8s.io/sample-controller v0.20.1
