#/bin/sh
set -e

../go-mod-hack.sh v1.20.1
go get github.com/argoproj/argo-cd@a4ee25b
