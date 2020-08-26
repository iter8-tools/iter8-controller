#!/usr/bin/env bash

# Exit on error
#set -e

# This test cases tests the following:
#   - experiment pauses when no baseline
#   - experiment proceeds immediately if both defined
#   - pause and resume
#   - rollback and rollforward

THIS=`basename $0`
DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

YAML_PATH=$DIR/../data/bookinfo
NAMESPACE="${NAMESPACE:-bookinfo-iter8}"
IP="${IP:-127.0.0.1}"
EXPERIMENT="${EXPERIMENT:-canary-reviews-v2v3-nobaseline}"
ANALYTICS_ENDPOINT="${ANALYTICS_ENDPOINT:-http://iter8-analytics:8080}"

header "Start iter8 end-to-end testing"

header "Scenario 1 - successful, no baseline, uniform, manual"

# cleanup any existing load generation, experiments, deployments
header "Clean Up Any Existing"
# delete any load generation
ps -aef | grep watch | grep -e 'curl.*bookinfo.example.com' | awk '{print $2}' | xargs kill
# delete any existing experiment with same name
kubectl -n $NAMESPACE delete experiment $EXPERIMENT $EXPERIMENT-restart --ignore-not-found
# delete any existing candidates
kubectl -n $NAMESPACE delete deployment reviews-v3 --ignore-not-found

header "Create $NAMESPACE namespace"
kubectl apply -f $YAML_PATH/namespace.yaml

header "Create $NAMESPACE app"
kubectl apply -n $NAMESPACE -f $YAML_PATH/bookinfo-tutorial.yaml
sleep 2
kubectl wait --for=condition=Ready pods --all -n $NAMESPACE --timeout=540s
kubectl get pods,services -n $NAMESPACE

header "Create $NAMESPACE gateway and vs"
kubectl apply -n $NAMESPACE -f $YAML_PATH/bookinfo-gateway.yaml
kubectl get gateway -n $NAMESPACE
kubectl get vs -n $NAMESPACE

header "Generate workload"
# We are using nodeport of the Istio ingress gateway to access bookinfo app
PORT=`kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}'`
# Following uses the K8s service IP/port to access bookinfo app
echo "Bookinfo is accessed at $IP:$PORT"
curl -H "Host: bookinfo.example.com" -Is "http://$IP:$PORT/productpage"
watch -n 0.1 "curl -H \"Host: bookinfo.example.com\" -Is \"http://$IP:$PORT/productpage\"" >/dev/null 2>&1 &

# baseline (reviews-v2) was deployed as part of application
# To test that iter8 pauses waiting for baseline, delete baseline first
header "Delete baseline version"
kubectl -n $NAMESPACE delete deployment reviews-v2

# And deploy canary version so it isn't the reason we pause
header "Deploy canary version"
yq w $YAML_PATH/reviews-v3.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl apply -n $NAMESPACE -f -
kubectl -n $NAMESPACE wait --for=condition=Ready pods  --selector="iter8/e2e-test=$THIS" --timeout=540s
kubectl get pods,services -n $NAMESPACE

# create experiment
# verify it is paused, waiting for baseline
header "Create Iter8 Experiment"
yq w $YAML_PATH/canary/canary_reviews-v2_to_reviews-v3.yaml spec.duration.interval 15s \
  | yq w - spec.duration.maxIterations 4 \
  | yq w - spec.analyticsEndpoint $ANALYTICS_ENDPOINT \
  | yq w - metadata.name $EXPERIMENT \
  | kubectl -n $NAMESPACE apply -f -
sleep 2
kubectl get experiments.iter8.tools -n $NAMESPACE
test_experiment_status $EXPERIMENT "TargetsError: Missing Baseline"

# (re)deploy baseline
# verify experiment starts
header "Deploy baseline version"
yq w $YAML_PATH/reviews-v2.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl apply -n $NAMESPACE -f -
kubectl -n $NAMESPACE wait --for=condition=Ready pods  --selector="iter8/e2e-test=$THIS" --timeout=540s
kubectl get pods,services -n $NAMESPACE
sleep 5
test_experiment_status $EXPERIMENT "IterationUpdate: Iteration"

# pause experiment
# verify experiment paused
header "Pause experiment"
kubectl -n $NAMESPACE patch experiments.iter8.tools $EXPERIMENT --type='merge' \
  -p='{"spec": {"manualOverride": { "action": "pause" }}}'
sleep 2
test_experiment_status $EXPERIMENT "ActionPause"
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml

# resume experiment
# verify experiment resumed
header "Resume experiment"
sleep 10
kubectl -n $NAMESPACE patch experiments.iter8.tools $EXPERIMENT --type='merge' \
  -p='{"spec": {"manualOverride": { "action": "resume" }}}'
sleep 10
test_experiment_status $EXPERIMENT "IterationUpdate: Iteration"
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml

# force experiment rollback
# verify experiment rolled back
header "rollback experiment"
kubectl -n $NAMESPACE patch experiments.iter8.tools $EXPERIMENT --type='merge' \
  -p='{"spec": {"manualOverride": { "action": "terminate", "trafficSplit": { "reviews-v2": 100, "reviews-v3": 0 }}}}'
sleep 2
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT
test_experiment_status $EXPERIMENT "ExperimentCompleted: Traffic User Specified"
kubectl -n $NAMESPACE get virtualservice.networking.istio.io reviews.$NAMESPACE.svc.cluster.local.iter8router -o yaml | yq r - spec.http
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 0 100
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 1 0

# restart experiment
# verify experiment started
header "restart experiment"
yq w $DIR/../data/bookinfo/canary/canary_reviews-v2_to_reviews-v3.yaml spec.duration.interval 15s \
  | yq w - spec.duration.maxIterations 10 \
  | yq w - spec.analyticsEndpoint $ANALYTICS_ENDPOINT \
  | yq w - metadata.name $EXPERIMENT-restart \
  | kubectl -n $NAMESPACE apply -f -
sleep 10
test_experiment_status "$EXPERIMENT-restart" "IterationUpdate: Iteration"
# kubectl -n $NAMESPACE get experiments.iter8.tools

# force experiment complete
# verify experiment complete
header "rollforward experiment"
kubectl -n $NAMESPACE patch experiments.iter8.tools "$EXPERIMENT-restart" --type='merge' \
  -p='{"spec": {"manualOverride": { "action": "terminate", "trafficSplit": { "reviews-v2": 0, "reviews-v3": 100 }}}}'
sleep 2
kubectl -n $NAMESPACE get experiments.iter8.tools "$EXPERIMENT-restart"
test_experiment_status "$EXPERIMENT-restart" "ExperimentCompleted: Traffic User Specified"
kubectl -n $NAMESPACE get virtualservice.networking.istio.io reviews.$NAMESPACE.svc.cluster.local.iter8router -o yaml | yq r - spec.http
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 0 0
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 1 100

header "Wait for experiment"
kubectl wait --for=condition=ExperimentCompleted -n $NAMESPACE experiments.iter8.tools reviews-v3-rollout --timeout=540s
kubectl get experiments.iter8.tools -n $NAMESPACE

echo "Experiment succeeded as expected!"

header "Clean up"
kubectl -n $NAMESPACE delete deployment reviews-v2 --ignore-not-found
