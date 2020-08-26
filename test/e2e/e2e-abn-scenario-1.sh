#!/usr/bin/env bash

# Exit on error
#set -e

THIS=`basename $0`
DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

YAML_PATH=$DIR/../data/bookinfo
ISTIO_NAMESPACE="${ISTIO_NAMESPACE:-istio-system}"
NAMESPACE="${NAMESPACE:-bookinfo-iter8}"
IP="${IP:-127.0.0.1}"
EXPERIMENT="${EXPERIMENT:-abn_productpage-v1v2v3}"
ANALYTICS_ENDPOINT="${ANALYTICS_ENDPOINT:-http://iter8-analytics:8080}"

header "Iter8 e2e Test Case(s)"

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

header "Scenario - A/B/n"

header "Set Up"

if [[ -n $ISOLATED_TEST ]]; then
  header "Kill Existing Load Generation"
  ps -aef | grep watch | grep -e 'curl.*bookinfo.example.com' | awk '{print $2}' | xargs kill
fi

header "Clean Up Any Existing"
# delete any existing experiment with same name
kubectl -n $NAMESPACE delete experiments.iter8.tools $EXPERIMENT --ignore-not-found
# delete any existing candidates
kubectl -n $NAMESPACE delete deployment productpage-v2 productpage-v3 --ignore-not-found

header "Create Iter8 Custom Metric"
if [ "$MIXER_DISABLED" = "false" ]; then
  echo "Using Istio telemetry v1"
  kubectl apply -n iter8 -f $YAML_PATH/abn/productpage-metrics-v1.yaml
else
  echo "Using Istio telemetry v2"
  kubectl apply -n iter8 -f $YAML_PATH/abn/productpage-metrics.yaml
fi
kubectl get configmap iter8config-metrics -n iter8 -oyaml

header "Create $NAMESPACE namespace"
kubectl apply -f $YAML_PATH/namespace.yaml

header "Create $NAMESPACE app"
kubectl apply --namespace $NAMESPACE -f $YAML_PATH/bookinfo-tutorial.yaml
sleep 1
if [[ -n $ISOLATED_TEST ]]; then
  # Travis seems slow to terminate pods so this is dangerous
  kubectl  --namespace $NAMESPACE wait --for=condition=Ready pods --all --timeout=540s
  if (( $? )); then echo "FAIL: application pods not started as expected"; exit 1; fi
fi
kubectl --namespace $NAMESPACE get pods,services

header "Create $NAMESPACE gateway and virtualservice"
kubectl --namespace $NAMESPACE apply -f $YAML_PATH/bookinfo-gateway.yaml
kubectl --namespace $NAMESPACE get gateway,virtualservice,destinationrule

if [[ -n $ISOLATED_TEST ]]; then
  header "Generate workload"
  # We are using nodeport of the Istio ingress gateway to access bookinfo app
  PORT=`kubectl --namespace $ISTIO_NAMESPACE get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}'`
  # Following uses the K8s service IP/port to access bookinfo app
  echo "Bookinfo is accessed at $IP:$PORT"
  echo "curl -H \"Host: bookinfo.example.com\" -Is \"http://$IP:$PORT/productpage\""
  curl -H "Host: bookinfo.example.com" -Is "http://$IP:$PORT/productpage"
  watch -n 0.1 "curl -H \"Host: bookinfo.example.com\" -Is \"http://$IP:$PORT/productpage\"" >/dev/null 2>&1 &
fi

# start experiment
# verify waiting for candidate
header "Create Iter8 Experiment"
yq w $YAML_PATH/abn/abn_productpage_v1v2v3.yaml metadata.name $EXPERIMENT \
  | yq w - spec.analyticsEndpoint $ANALYTICS_ENDPOINT \
  | kubectl --namespace $NAMESPACE apply -f -
sleep 2
kubectl get experiments.iter8.tools -n $NAMESPACE
test_experiment_status $EXPERIMENT "TargetsError: Missing Candidate"

# start candidate versions
# verify experiment progressing
header "Deploy candidate versions"
yq w $YAML_PATH/productpage-v2.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl --namespace $NAMESPACE apply -f -
yq w $YAML_PATH/productpage-v3.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl --namespace $NAMESPACE apply -f -
kubectl --namespace $NAMESPACE wait --for=condition=Ready pods  --selector="iter8/e2e-test=$THIS" --timeout=540s
if (( $? )); then echo "FAIL: candidate pods not started as expected"; exit 1; fi
kubectl --namespace $NAMESPACE get pods,services
sleep 2
test_experiment_status $EXPERIMENT "IterationUpdate: Iteration"
# kubectl --namespace $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml

# wait for experiment to complete
kubectl --namespace $NAMESPACE wait --for=condition=ExperimentCompleted experiments.iter8.tools $EXPERIMENT --timeout=540s
kubectl --namespace $NAMESPACE get experiments.iter8.tools

header "Test results"
# kubectl --namespace $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml
test_experiment_status $EXPERIMENT "ExperimentCompleted: Traffic To Winner"
test_vs_percentages bookinfo 0 0
test_vs_percentages bookinfo 1 0
test_vs_percentages bookinfo 2 100

echo "Experiment succeeded as expected!"

header "Clean up"
kubectl --namespace $NAMESPACE delete deployment productpage-v2 productpage-v3
