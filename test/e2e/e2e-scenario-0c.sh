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

echo " NAMESPACE = $NAMESPACE"
echo "        IP = $IP"
echo "EXPERIMENT = $EXPERIMENT"

header "Start iter8 end-to-end testing"

header "Scenario 0c - pause, resume"

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
if [[ -n $ISOLATED_TEST ]]; then
  # Travis seems slow to terminate pods so this is dangerous
  kubectl wait --for=condition=Ready pods --all -n $NAMESPACE --timeout=540s
fi
kubectl get pods,services -n $NAMESPACE

header "Create $NAMESPACE gateway and vs"
kubectl apply -n $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/bookinfo-gateway.yaml
kubectl get gateway -n $NAMESPACE
kubectl get vs -n $NAMESPACE

if [[ -n $ISOLATED_TEST ]]; then
  header "Generate workload"
  # We are using nodeport of the Istio ingress gateway to access bookinfo app
  PORT=`kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}'`
  # Following uses the K8s service IP/port to access bookinfo app
  echo "Bookinfo is accessed at $IP:$PORT"
  curl -H "Host: bookinfo.example.com" -Is "http://$IP:$PORT/productpage"
  watch -n 0.1 "curl -H \"Host: bookinfo.example.com\" -Is \"http://$IP:$PORT/productpage\"" >/dev/null 2>&1 &
fi

# start experiment
# verify waiting for candidate
header "Create Iter8 Experiment"
yq w $DIR/../../doc/tutorials/istio/bookinfo/canary_reviews-v2_to_reviews-v3.yaml spec.duration.interval 15s \
  | yq w - spec.duration.maxIterations 10 \
  | yq w - spec.analyticsEndpoint $ANALYTICS_ENDPOINT \
  | yq w - metadata.name $EXPERIMENT \
  | kubectl -n $NAMESPACE apply -f -
sleep 2
kubectl -n $NAMESPACE get experiments.iter8.tools
test_experiment_status $EXPERIMENT "TargetsError: Err in getting candidates:"

# start canary
# verify experiment progressing
header "Deploy canary version"
yq w $DIR/../../doc/tutorials/istio/bookinfo/reviews-v3.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl apply -n $NAMESPACE -f -
kubectl -n $NAMESPACE wait --for=condition=Ready pods  --selector="iter8/e2e-test=$THIS" --timeout=540s
kubectl get pods,services -n $NAMESPACE
sleep 2
test_experiment_status $EXPERIMENT "IterationUpdate: Iteration"
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml

# pause experiment
# verify experiment paused
header "pause experiment"
kubectl -n $NAMESPACE patch experiments.iter8.tools $EXPERIMENT --type='merge' \
  -p='{"spec": {"manualOverride": { "action": "pause" }}}'
sleep 2
test_experiment_status $EXPERIMENT "ActionPause"
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml

# resume experiment
# verify experiment resumed
header "resume experiment"
kubectl -n $NAMESPACE patch experiments.iter8.tools $EXPERIMENT --type='merge' \
  -p='{"spec": {"manualOverride": { "action": "resume" }}}'
sleep 20
test_experiment_status $EXPERIMENT "IterationUpdate: Iteration"
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml

# # force experiment rollback
# # verify experiment rolled back
# header "rollback experiment"
# kubectl -n $NAMESPACE patch experiments.iter8.tools $EXPERIMENT --type='merge' \
#   -p='{"spec": {"manualOverride": { "action": "terminate", "trafficSplit": { "reviews-v2": 100, "reviews-v3": 0 }}}}'
# sleep 2
# test_experiment_status $EXPERIMENT "IterationUpdate: Iteration"
# kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml

# # restart experiment
# # verify experiment started
# header "restart experiment"
# yq w $DIR/../../doc/tutorials/istio/bookinfo/canary_reviews-v2_to_reviews-v3.yaml spec.duration.interval 15s \
#   | yq w - spec.duration.maxIterations 20 \
#   | yq w - spec.analyticsEndpoint http://127.0.0.1:8080/assessment \
#   | yq w - metadata.name "$EXPERIMENT-restart" \
#   | kubectl -n $NAMESPACE apply -f -
# sleep 2
# test_experiment_status "$EXPERIMENT-restart" "IterationUpdate: Iteration"
# kubectl -n $NAMESPACE get experiments.iter8.tools

# # force experiment complete
# # verify experiment complete
# header "rollforward experiment"
# kubectl -n $NAMESPACE patch experiments.iter8.tools $EXPERIMENT --type='merge' \
#   -p='{"spec": {"manualOverride": { "action": "terminate", "trafficSplit": { "reviews-v2": 0, "reviews-v3": 100 }}}}'
# sleep 2
# test_experiment_status $EXPERIMENT "IterationUpdate: Iteration"
# kubectl -n $NAMESPACE get experiments.iter8.tools "$EXPERIMENT-restart" -o yaml

echo "Experiment succeeded as expected!"

header "Clean up"
kubectl -n $NAMESPACE delete deployment reviews-v2
