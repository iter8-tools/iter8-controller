#!/usr/bin/env bash
#
# Copyright 2019 IBM Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
set -e

ROOT=$(dirname $0)
source $ROOT/../scripts/library.sh

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

set -o errtrace
trap traperr ERR
trap traperr INT

parse_flags $*

if [ -z "$SKIP_SETUP" ]
then
    setup_knative
fi

export NAMESPACE=$(random_namespace)
header "creating namespace $NAMESPACE"
kubectl create ns $NAMESPACE

header "install iter8 CRDs"
make install

header "build iter8 controller"
mkdir -p bin
go build -o bin/manager ./cmd/manager/main.go
chmod +x bin/manager

header "deploy metrics configmap"
kubectl apply -f install/helm/iter8-controller/templates/metrics/iter8_metrics.yaml -n $NAMESPACE

header "run iter8 controller locally"
./bin/manager &
CONTROLLER_PID=$!
echo "controller started $CONTROLLER_PID"

sleep 4 # wait for controller to start

go test -v -p 1 ./test/e2e/... -args -namespace ${NAMESPACE}

cleanup
