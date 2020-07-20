#!/usr/bin/env bash

# Exit on error
#set -e

THIS=`basename $0`
DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

YAML_PATH=$DIR/../../doc/tutorials/istio/bookinfo/kubecon
ISTIO_NAMESPACE="${ISTIO_NAMESPACE:-istio-system}"
NAMESPACE="${NAMESPACE:-kubecon-demo}"
IP="${IP:-127.0.0.1}"
EXPERIMENT="${EXPERIMENT:-productpage-abn-test}"
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

header "Scenario - kubecon demo"

header "Set Up"

header "Clean Up Any Existing"
# delete any existing experiment with same name
kubectl -n $NAMESPACE delete experiment $EXPERIMENT --ignore-not-found
# delete any existing candidates
kubectl -n $NAMESPACE delete -f deployment productpage-v2 productpage-v3 --ignore-not-found

header "Create Iter8 Custom Metric"
if [ "$MIXER_DISABLED" = "false" ]; then
  echo "Using Istio telemetry v1"
  kubectl apply -n iter8 -f $YAML_PATH/kc-configmap-telemetry-v1.yaml
else
  echo "Using Istio telemetry v2"
  kubectl apply -n iter8 -f $YAML_PATH/kc-configmap-telemetry-v2.yaml
fi
kubectl get configmap iter8config-metrics -n iter8 -oyaml

header "Create $NAMESPACE namespace"
kubectl apply -f $YAML_PATH/kc-namespace.yaml

header "Create $NAMESPACE app"
kubectl apply --namespace $NAMESPACE -f $YAML_PATH/kc-bookinfo-tutorial.yaml
sleep 1
if [[ -n $ISOLATED_TEST ]]; then
  # Travis seems slow to terminate pods so this is dangerous
  kubectl  --namespace $NAMESPACE wait --for=condition=Ready pods --all --timeout=540s
fi
kubectl --namespace $NAMESPACE get pods,services

header "Create $NAMESPACE gateway and virtualservice"
kubectl --namespace $NAMESPACE apply -f $YAML_PATH/kc-bookinfo-gateway.yaml
kubectl --namespace $NAMESPACE get gateway,virtualservice

if [[ -n $ISOLATED_TEST ]]; then
  header "Generate workload"
  # We are using nodeport of the Istio ingress gateway to access bookinfo app
  PORT=`kubectl --namespace $ISTIO_NAMESPACE get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}'`
  # Following uses the K8s service IP/port to access bookinfo app
  echo "Bookinfo is accessed at $IP:$PORT"
  echo "curl -H \"Host: bookinfo-kubecon.example.com\" -Is \"http://$IP:$PORT/productpage\""
  curl -H "Host: bookinfo-kubecon.example.com" -Is "http://$IP:$PORT/productpage"
  watch -n 0.1 "curl -H \"Host: bookinfo-kubecon.example.com\" -Is \"http://$IP:$PORT/productpage\"" >/dev/null 2>&1 &
fi

# start experiment
# verify waiting for candidate
header "Create Iter8 Experiment"
yq w $YAML_PATH/kc-experiment.yaml metadata.name $EXPERIMENT \
  | yq w - spec.analyticsEndpoint $ANALYTICS_ENDPOINT \
  | kubectl --namespace $NAMESPACE apply -f -
sleep 2
kubectl get experiments.iter8.tools -n $NAMESPACE
test_experiment_status $EXPERIMENT "TargetsError: Err in getting candidates:"

# start candidat versions
# verify experiment progressing
header "Deploy candidate versions"
yq w $YAML_PATH/kc-productpage-v2.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl --namespace $NAMESPACE apply -f -
yq w $YAML_PATH/kc-productpage-v3.yaml spec.template.metadata.labels[iter8/e2e-test] $THIS \
  | kubectl --namespace $NAMESPACE apply -f -
kubectl --namespace $NAMESPACE wait --for=condition=Ready pods  --selector="iter8/e2e-test=$THIS" --timeout=540s
kubectl --namespace $NAMESPACE get pods,services
sleep 2
test_experiment_status $EXPERIMENT "IterationUpdate: Iteration"
kubectl --namespace $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml

# wait for experiment to complete
kubectl --namespace $NAMESPACE wait --for=condition=ExperimentCompleted experiments.iter8.tools reviews-v3-rollout --timeout=540s
kubectl --namespace $NAMESPACEget experiments.iter8.tools

header "Test results"
kubectl --namespace $NAMESPACE get experiments.iter8.tools $EXPERIMENT -o yaml
test_experiment_status $EXPERIMENT "ExperimentCompleted: Last Iteration Was Completed"

echo "Experiment succeeded as expected!"

header "Clean up"
kubectl --namespace $NAMESPACE delete deployment productpage-v2 productpage-v3
