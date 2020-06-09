#!/usr/bin/env bash

# Exit on error
set -e

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

header "Scenario 2"

header "Create Iter8 Experiment"
kubectl apply -n bookinfo-iter8 -f $DIR/../../doc/tutorials/istio/bookinfo/canary_reviews-v3_to_reviews-v4.yaml
kubectl get experiments -n bookinfo-iter8

header "Deploy canary version"
kubectl apply -n bookinfo-iter8 -f $DIR/../../doc/tutorials/istio/bookinfo/reviews-v4.yaml
sleep 1
kubectl wait --for=condition=ExperimentCompleted -n bookinfo-iter8 experiments.iter8.tools reviews-v4-rollout --timeout=600s
kubectl get experiments -n bookinfo-iter8

header "Test results"
kubectl -n bookinfo-iter8 get experiments.iter8.tools reviews-v4-rollout -o yaml
conclusion=`kubectl -n bookinfo-iter8 get experiments.iter8.tools reviews-v4-rollout -o=jsonpath='{.status.assessment.conclusions[0]}'`
if [ "$conclusion" = "All success criteria were  met" ]; then
  echo "Experiment succeeded unexpectedly!"
  exit 1
fi
echo "Experiment failed as expected!"
