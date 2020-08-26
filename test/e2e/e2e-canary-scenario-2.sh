#!/usr/bin/env bash

# Exit on error
#set -e

# This test case tests an end to end canary cases:
#    - experiment pauses when no candidate
#    - complete successful canary 
#    - failuer due to latency metric
#    - failure due to error metric
#    - cleanup

THIS=`basename $0`
DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

YAML_PATH=$DIR/../data/bookinfo
NAMESPACE="${NAMESPACE:-bookinfo-iter8}"
IP="${IP:-127.0.0.1}"
EXPERIMENT="${EXPERIMENT:-canary-reviews}"
ANALYTICS_ENDPOINT="${ANALYTICS_ENDPOINT:-http://iter8-analytics:8080}"

header "Start iter8 end-to-end testing"

header "Scenario 2 - success + fail, no candidate, both, progressive"

# cleanup any existing load generation, experiments, deployments
header "Clean Up Any Existing"
# delete any load generation
ps -aef | grep watch | grep -e 'curl.*bookinfo.example.com' | awk '{print $2}' | xargs kill
# delete any existing experiment with same name
kubectl -n $NAMESPACE delete experiment $EXPERIMENT-v2v3 $EXPERIMENT-v3v4 $EXPERIMENT-v4v5 --ignore-not-found
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

# start successful experiment
# verify waiting for candidate
header "Create Iter8 Experiment"
yq w $YAML_PATH/canary/canary_reviews-v2_to_reviews-v3.yaml spec.duration.interval 15s \
  | yq w - spec.duration.maxIterations 4 \
  | yq w - spec.analyticsEndpoint $ANALYTICS_ENDPOINT \
  | yq w - metadata.name $EXPERIMENT-v2v3 \
  | yq w - spec.cleanup true \
  | kubectl -n $NAMESPACE apply -f -
sleep 2
kubectl get experiments.iter8.tools -n $NAMESPACE
test_experiment_status $EXPERIMENT-v2v3 "TargetsError: Missing Candidate"

# start canary
# verify experiment progressing
header "Deploy canary version"
yq w $YAML_PATH/reviews-v3.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl apply -n $NAMESPACE -f -
kubectl -n $NAMESPACE wait --for=condition=Ready pods  --selector="iter8/e2e-test=$THIS" --timeout=540s
kubectl get pods,services -n $NAMESPACE
sleep 2
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT-v2v3
test_experiment_status $EXPERIMENT-v2v3 "IterationUpdate: Iteration"

# wait for experiment to complete
kubectl wait --for=condition=ExperimentCompleted -n $NAMESPACE experiments.iter8.tools $EXPERIMENT-v2v3 --timeout=540s
kubectl -n $NAMESPACE get experiments.iter8.tools 

# test results of experiment; candidate should be winner; baseline deleted
header "Test results of $EXPERIMENT-v2v3"
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT-v2v3
test_experiment_status $EXPERIMENT-v2v3 "ExperimentCompleted: Traffic To Winner"
test_winner_found $EXPERIMENT-v2v3 true
test_current_best $EXPERIMENT-v2v3 reviews-v3
test_deployment_deleted reviews-v2
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 0 0
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 1 100

#
#
#

# deploy failing version (reviews-v4)
# start canary
header "Deploy canary version"
yq w $YAML_PATH/reviews-v4.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl apply -n $NAMESPACE -f -
kubectl -n $NAMESPACE wait --for=condition=Ready pods  --selector="iter8/e2e-test=$THIS" --timeout=540s
kubectl get pods,services -n $NAMESPACE
sleep 2
# start failing experiment
# verify starts immediately
header "Create Iter8 Experiment"
yq w $YAML_PATH/canary/canary_reviews-v3_to_reviews-v4.yaml spec.duration.interval 15s \
  | yq w - spec.duration.maxIterations 4 \
  | yq w - spec.analyticsEndpoint $ANALYTICS_ENDPOINT \
  | yq w - metadata.name $EXPERIMENT-v3v4 \
  | yq w - spec.cleanup true \
  | kubectl -n $NAMESPACE apply -f -
sleep 2
kubectl get experiments.iter8.tools -n $NAMESPACE
test_experiment_status $EXPERIMENT-v3v4 "IterationUpdate: Iteration"

# wait for experiment to complete
kubectl wait --for=condition=ExperimentCompleted -n $NAMESPACE experiments.iter8.tools $EXPERIMENT-v3v4 --timeout=540s
kubectl get experiments.iter8.tools -n $NAMESPACE

# test results of experiment: baseline should be winner; candidate deleted
header "Test results of $EXPERIMENT-v3v4"
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT-v3v4
test_experiment_status $EXPERIMENT-v3v4 "ExperimentCompleted: Traffic To Winner"
test_winner_found $EXPERIMENT-v3v4 true
test_current_best $EXPERIMENT-v3v4 reviews-v3
test_deployment_deleted reviews-v4
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 0 100
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 1 0

#
#
#

# start another failing experiment
header "Create Iter8 Experiment"
yq w $YAML_PATH/canary/canary_reviews-v3_to_reviews-v5.yaml spec.duration.interval 15s \
  | yq w - spec.duration.maxIterations 4 \
  | yq w - spec.analyticsEndpoint $ANALYTICS_ENDPOINT \
  | yq w - metadata.name $EXPERIMENT-v3v5 \
  | yq w - spec.cleanup true \
  | kubectl -n $NAMESPACE apply -f -
sleep 2
kubectl get experiments.iter8.tools -n $NAMESPACE

# start failing version (reviews-v5)
header "Deploy canary version"
yq w $YAML_PATH/reviews-v5.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl apply -n $NAMESPACE -f -
kubectl -n $NAMESPACE wait --for=condition=Ready pods  --selector="iter8/e2e-test=$THIS" --timeout=540s
kubectl get pods,services -n $NAMESPACE
sleep 2

# verify test running
test_experiment_status $EXPERIMENT-v3v5 "IterationUpdate: Iteration"

# wait for experiment to complete
kubectl wait --for=condition=ExperimentCompleted -n $NAMESPACE experiments.iter8.tools $EXPERIMENT-v3v5 --timeout=540s

# test results of experiment: baseline should be winner; candidate deleted
header "Test results of $EXPERIMENT-v3v5"
kubectl -n $NAMESPACE get experiments.iter8.tools $EXPERIMENT-v3v5
test_experiment_status $EXPERIMENT-v3v5 "ExperimentCompleted: Traffic To Winner"
test_winner_found $EXPERIMENT-v3v5 true
test_current_best $EXPERIMENT-v3v5 reviews-v3
test_deployment_deleted reviews-v5
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 0 100
test_vs_percentages reviews.$NAMESPACE.svc.cluster.local.iter8router 1 0

echo "Experiment succeeded as expected!"

header "Clean up"
kubectl -n $NAMESPACE delete deployment reviews-v2 reviews-v4 reviews-v5 --ignore-not-found
