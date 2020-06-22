#!/usr/bin/env bash

# Exit on error
#set -e

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

# This scenario reuses the Istio Gateway and Virtual Service created in scenario 1

header "Scenario 5"

header "Create Iter8 Experiment"
kubectl apply -n bookinfo-iter8 -f $DIR/../../doc/tutorials/istio/bookinfo/canary_productpage-v1_to_productpage-v2.yaml
kubectl get experiments -n bookinfo-iter8

header "Deploy canary version"
kubectl apply -n bookinfo-iter8 -f $DIR/../../doc/tutorials/istio/bookinfo/productpage-v2.yaml
sleep 1
kubectl wait --for=condition=ExperimentCompleted -n bookinfo-iter8 experiments.iter8.tools productpage-v2-rollout --timeout=540s
kubectl get experiments -n bookinfo-iter8
kubectl get vs bookinfo -n bookinfo-iter8 -o yaml

header "Test results"
kubectl -n bookinfo-iter8 get experiments.iter8.tools productpage-v2-rollout -o yaml
conclusion=`kubectl -n bookinfo-iter8 get experiments.iter8.tools productpage-v2-rollout -o=jsonpath='{.status.assessment.conclusions[0]}'`
if [ "$conclusion" != "All success criteria were  met" ]; then
  echo "Experiment failed unexpectedly!"
  exit 1
fi
echo "Experiment succeeded as expected!"

header "Clean up"
kubectl -n bookinfo-iter8 delete deployment productpage-v1
