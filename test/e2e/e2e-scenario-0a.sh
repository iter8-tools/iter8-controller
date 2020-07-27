#!/usr/bin/env bash

# Exit on error
#set -e

THIS=`basename $0`
DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

NAMESPACE="${NAMESPACE:-bookinfo-iter8}"
IP="${IP:-127.0.0.1}"
EXPERIMENT="${EXPERIMENT:-reviews-v3-rollout}"
ANALYTICS_ENDPOINT="${ANALYTICS_ENDPOINT:-http://iter8-analytics:8080}"

header "Start iter8 end-to-end testing"

header "Scenario 0a - pre-existing deployments - uniform"

header "Set Up"

header "Clean Up Any Existing"
# delete any existing experiment with same name
kubectl -n $NAMESPACE delete experiment $EXPERIMENT --ignore-not-found
# delete any existing candidates
kubectl -n $NAMESPACE delete deployment reviews-v3 --ignore-not-found

header "Create $NAMESPACE namespace"
kubectl apply -f $DIR/../../doc/tutorials/istio/bookinfo/namespace.yaml

header "Create $NAMESPACE app"
kubectl apply -n $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/bookinfo-tutorial.yaml
sleep 1
kubectl wait --for=condition=Ready pods --all -n $NAMESPACE --timeout=540s
kubectl get pods,services -n $NAMESPACE

header "Create $NAMESPACE gateway and vs"
kubectl apply -n $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/bookinfo-gateway.yaml
kubectl get gateway -n $NAMESPACE
kubectl get vs -n $NAMESPACE

header "Generate workload"
# We are using nodeport of the Istio ingress gateway to access bookinfo app
PORT=`kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}'`
# Following uses the K8s service IP/port to access bookinfo app
echo "Bookinfo is accessed at $IP:$PORT"
curl -H "Host: bookinfo.example.com" -Is "http://$IP:$PORT/productpage"
watch -n 0.1 "curl -H \"Host: bookinfo.example.com\" -Is \"http://$IP:$PORT/productpage\"" >/dev/null 2>&1 &

header "Deploy canary version"
yq w $DIR/../../doc/tutorials/istio/bookinfo/reviews-v3.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl apply -n $NAMESPACE -f -
kubectl -n $NAMESPACE wait --for=condition=Ready pods  --selector="iter8/e2e-test=$THIS" --timeout=540s
kubectl get pods,services -n $NAMESPACE

# create experiment
# verify it is progressing
header "Create Iter8 Experiment"
yq w $DIR/../../doc/tutorials/istio/bookinfo/canary_reviews-v2_to_reviews-v3.yaml spec.duration.interval 15s \
  | yq w - spec.duration.maxIterations 4 \
  | yq w - spec.analyticsEndpoint $ANALYTICS_ENDPOINT \
  | yq w - metadata.name $EXPERIMENT \
  | kubectl -n $NAMESPACE apply -f -
sleep 2
kubectl get experiments.iter8.tools -n $NAMESPACE
test_experiment_status $EXPERIMENT "IterationUpdate: Iteration"

header "Wait for experiment"
kubectl wait --for=condition=ExperimentCompleted -n $NAMESPACE experiments.iter8.tools reviews-v3-rollout --timeout=540s
kubectl get experiments.iter8.tools -n $NAMESPACE

header "Test results"
kubectl -n $NAMESPACE get experiments.iter8.tools reviews-v3-rollout -o yaml
test_experiment_status $EXPERIMENT "ExperimentCompleted: Traffic To Winner"

echo "Experiment succeeded as expected!"

header "Clean up"
kubectl -n $NAMESPACE delete deployment reviews-v2
