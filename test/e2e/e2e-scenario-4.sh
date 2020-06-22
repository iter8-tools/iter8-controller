#!/usr/bin/env bash

# Exit on error
#set -e

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

header "Scenario 4"

header "Create Iter8 Custom Metric"
kubectl apply -n iter8 -f $DIR/../../doc/tutorials/istio/bookinfo/iter8_metrics_extended.yaml
kubectl get configmap iter8config-metrics -n iter8 -oyaml

header "Create Iter8 Experiment"
kubectl apply -n bookinfo-iter8 -f $DIR/../../doc/tutorials/istio/bookinfo/canary_reviews-v3_to_reviews-v6.yaml
kubectl get experiments -n bookinfo-iter8

header "Deploy canary version"
kubectl apply -n bookinfo-iter8 -f $DIR/../../doc/tutorials/istio/bookinfo/reviews-v6.yaml
sleep 1
kubectl wait --for=condition=ExperimentCompleted -n bookinfo-iter8 experiments.iter8.tools reviews-v6-rollout --timeout=540s
kubectl get experiments -n bookinfo-iter8

header "Test results"
kubectl -n bookinfo-iter8 get experiments.iter8.tools reviews-v6-rollout -o yaml
conclusion=`kubectl -n bookinfo-iter8 get experiments.iter8.tools reviews-v6-rollout -o=jsonpath='{.status.assessment.conclusions[0]}'`
if [ "$conclusion" != "All success criteria were  met" ]; then
  echo "Experiment failed unexpectedly!"
  exit 1
fi
echo "Experiment succeeded as expected!"

header "Clean up"
kubectl -n bookinfo-iter8 delete deployment reviews-v3
