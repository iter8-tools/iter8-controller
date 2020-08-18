#!/usr/bin/env bash

# Setting this might prevent cleanup() getting called
#set -e

# This only runs in the context of Travis (see .travis.yaml), where setup are done

ROOT=$(dirname $0)
source $ROOT/library.sh

function cleanup() {
  if [ -n "$NAMESPACE" ]
  then
    echo "deleting namespace $NAMESPACE"
    kubectl delete ns $NAMESPACE

    echo "namespace $NAMESPACE deleted"
    unset NAMESPACE
  fi

  # let the controller remove the finalizer
  if [ -n "$CONTROLLER_PID" ]
  then
    unset CONTROLLER_PID
    pkill main 
    echo "controller stops"
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
header "creating namespace $NAMESPACE"
kubectl create ns $NAMESPACE

header "deploy metrics configmap"
kubectl apply -f ./test/e2e/iter8_metrics_test.yaml -n $NAMESPACE

header "run iter8 controller locally"
make run &
CONTROLLER_PID=$!
echo "controller started $CONTROLLER_PID"

sleep 5 # wait for controller to start

go test -run TestExperiment -v -p 1 ./test/e2e/ -args -namespace ${NAMESPACE}

cleanup
