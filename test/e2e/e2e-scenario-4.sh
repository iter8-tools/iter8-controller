#!/usr/bin/env bash

# Exit on error
#set -e

ISTIO_NAMESPACE=istio-system

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

header "Scenario 4"

echo "Istio namespace: $ISTIO_NAMESPACE"
MIXER_DISABLED=`kubectl -n $ISTIO_NAMESPACE get cm istio -o json | jq .data.mesh | grep -o 'disableMixerHttpReports: [A-Za-z]\+' | cut -d ' ' -f2`
ISTIO_VERSION=`kubectl -n $ISTIO_NAMESPACE get pods -o yaml | grep "image:" | grep proxy | head -n 1 | awk -F: '{print $3}'`

if [ -z "$ISTIO_VERSION" ]; then
  echo "Cannot detect Istio version, aborting..."
  exit 1
elif [ -z "$MIXER_DISABLED" ]; then
  echo "Cannot detect Istio telemetry version, aborting..."
  exit 1
fi

echo "Istio version: $ISTIO_VERSION"
echo "Istio mixer disabled: $MIXER_DISABLED"

header "Create Iter8 Custom Metric"
if [ "$MIXER_DISABLED" = "false" ]; then
  echo "Using Istio telemetry v1"
  kubectl apply -n iter8 -f $DIR/../../doc/tutorials/istio/bookinfo/iter8_metrics_extended.yaml
else
  echo "Using Istio telemetry v2"
  kubectl apply -n iter8 -f $DIR/../../doc/tutorials/istio/bookinfo/iter8_metrics_extended_telemetry-v2.yaml
fi
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
