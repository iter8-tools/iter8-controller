#!/usr/bin/env bash

# Exit on error
#set -e

NAMESPACE=bookinfo-service

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

header "Scenario 5"

header "Create $NAMESPACE namespace"
kubectl create ns $NAMESPACE
kubectl label ns $NAMESPACE istio-injection=enabled

header "Create $NAMESPACE app"
kubectl apply -n $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/bookinfo-tutorial.yaml \
              -f $DIR/../../doc/tutorials/istio/bookinfo/service/productpage-v1.yaml
sleep 1
kubectl wait --for=condition=Ready pods --all -n $NAMESPACE --timeout=540s
kubectl get pods,services -n $NAMESPACE

header "Create Iter8 Experiment"
kubectl apply -n $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/service/canary_productpage-v1_to_productpage-v2.yaml
kubectl get experiments -n $NAMESPACE

header "Deploy canary version"
kubectl apply -n $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/productpage-v2.yaml \
              -f $DIR/../../doc/tutorials/istio/bookinfo/service/productpage-v2.yaml
sleep 1
kubectl wait --for=condition=ExperimentCompleted -n $NAMESPACE experiments.iter8.tools productpage-v2-rollout --timeout=540s
kubectl get experiments -n $NAMESPACE
kubectl get vs -n $NAMESPACE -o yaml

header "Test results"
kubectl -n $NAMESPACE get experiments.iter8.tools productpage-v2-rollout -o yaml
conclusion=`kubectl -n $NAMESPACE get experiments.iter8.tools productpage-v2-rollout -o=jsonpath='{.status.assessment.conclusions[0]}'`
if [ "$conclusion" != "All success criteria were  met" ]; then
  echo "Experiment failed unexpectedly!"
  exit 1
fi
echo "Experiment succeeded as expected!"

header "Clean up"
kubectl -n bookinfo-iter8 delete deployment productpage-v1
