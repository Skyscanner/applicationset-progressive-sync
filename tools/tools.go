// +build tools

// This file ensures tool dependencies are kept in sync.  This is the
// recommended way of doing this according to
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
// To install the following tools at the version used by this repo run:
// $ make tools
// or
// $ go generate -tags tools tools/tools.go

package tools

//go:generate go install github.com/onsi/ginkgo/ginkgo
import _ "github.com/onsi/ginkgo/ginkgo"

//go:generate go install sigs.k8s.io/controller-tools/cmd/controller-gen
import _ "sigs.k8s.io/controller-tools/cmd/controller-gen"
