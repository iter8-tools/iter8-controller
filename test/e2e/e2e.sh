#!/usr/bin/env bash

set -e

ROOT=$(dirname $0)

function cleanup() {
  if [ -n "$NAMESPACE" ]
  then
    echo "deleting namespace $NAMESPACE"
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

export NAMESPACE=$(random_namespace)
echo "creating namespace $NAMESPACE"
kubectl create ns $NAMESPACE
    
echo "install iter8 CRDs"
make load

echo "deploy metrics configmap"
kubectl apply -f ./test/e2e/iter8_metrics_test.yaml -n $NAMESPACE

echo "build iter8 controller"
mkdir -p bin
go build -o bin/manager ./cmd/manager/main.go
chmod +x bin/manager

echo "run iter8 controller locally"
./bin/manager &
CONTROLLER_PID=$!
echo "controller started $CONTROLLER_PID"

sleep 4 # wait for controller to start

go test -run TestKubernetesExperiment -v -p 1 ./test/e2e/ -args -namespace ${NAMESPACE}

cleanup
