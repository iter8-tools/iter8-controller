#!/usr/bin/env bash

# Setting this might prevent cleanup() getting called
#set -e

# This only runs in the context of Travis (see .travis.yaml), where setup are done

ROOT=$(dirname $0)
source $ROOT/../../iter8-trend/test/library.sh

function cleanup() {
  if [ -n "$NAMESPACE" ]
  then
    header "deleting namespace $NAMESPACE"
    kubectl delete ns $NAMESPACE
    unset NAMESPACE
  fi

  # let the controller remove the finalizer
  if [ -n "$CONTROLLER_PID" ]
  then
    kill $CONTROLLER_PID
    unset CONTROLLER_PID
  fi
}

function traperr() {
  echo "ERROR: ${BASH_SOURCE[1]} at about ${BASH_LINENO[0]}"
  cleanup
}

function random_namespace() {
  ns="iter8-testing-$(cat /dev/urandom | env LC_CTYPE=C tr -dc 'a-z0-9' | fold -w 6 | head -n 1)"
  echo $ns
}

set -o errtrace
trap traperr ERR
trap traperr INT

header "install kubectl"
curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/$KUBE_VERSION/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/

header "install helm"
curl -fsSL https://get.helm.sh/helm-v2.16.7-linux-amd64.tar.gz | tar xvzf - && sudo mv linux-amd64/helm /usr/local/bin

export NAMESPACE=$(random_namespace)
header "creating namespace $NAMESPACE"
kubectl create ns $NAMESPACE
    
header "install iter8 CRDs"
make load

header "deploy metrics configmap"
kubectl apply -f ./test/e2e/iter8_metrics_test.yaml -n $NAMESPACE

header "build iter8 controller"
mkdir -p bin
go build -o bin/manager ./cmd/manager/main.go
chmod +x bin/manager

header "run iter8 controller locally"
./bin/manager &
CONTROLLER_PID=$!
echo "controller started $CONTROLLER_PID"

sleep 4 # wait for controller to start

go test -run TestKubernetesExperiment -v -p 1 ./test/e2e/ -args -namespace ${NAMESPACE}

cleanup
