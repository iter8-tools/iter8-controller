#!/usr/bin/env bash

# Exit on error
#set -e

NAMESPACE=bookinfo-service

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

header "Scenario 5"

header "Create $NAMESPACE namespace"
kubectl create ns $NAMESPACE
kubectl label ns $NAMESPACE istio-injection=enabled

header "Create $NAMESPACE app"
kubectl apply -n $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/bookinfo-tutorial.yaml \
              -f $DIR/../../doc/tutorials/istio/bookinfo/service/productpage-v1.yaml
sleep 1
kubectl wait --for=condition=Ready pods --all -n $NAMESPACE --timeout=540s
kubectl get pods,services -n $NAMESPACE

header "Create $NAMESPACE gateway"
kubectl apply -n  $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/service/bookinfo-gateway.yaml
kubectl get gateway -n  $NAMESPACE

header "Generate workload"
# We are using nodeport of the Istio ingress gateway to access bookinfo app
IP='127.0.0.1'
PORT=`kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="http2")].nodePort}'`
# Following uses the K8s service IP/port to access bookinfo app
echo "Bookinfo is accessed at $IP:$PORT"
curl -H "Host: productpage.example.com" -Is "http://$IP:$PORT/productpage"
watch -n 0.1 "curl -H \"Host: productpage.example.com\" -Is \"http://$IP:$PORT/productpage\"" >/dev/null 2>&1 &

header "Create Iter8 Experiment"
kubectl apply -n $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/service/canary_productpage-v1_to_productpage-v2.yaml
kubectl get experiments -n $NAMESPACE

header "Deploy canary version"
kubectl apply -n $NAMESPACE -f $DIR/../../doc/tutorials/istio/bookinfo/productpage-v2.yaml \
              -f $DIR/../../doc/tutorials/istio/bookinfo/service/productpage-v2.yaml
sleep 1
kubectl wait --for=condition=ExperimentCompleted -n $NAMESPACE experiments.iter8.tools productpage-v2-rollout --timeout=540s
kubectl get experiments -n $NAMESPACE
kubectl get vs -n $NAMESPACE -o yaml

header "Test results"
kubectl -n $NAMESPACE get experiments.iter8.tools productpage-v2-rollout -o yaml
conclusion=`kubectl -n $NAMESPACE get experiments.iter8.tools productpage-v2-rollout -o=jsonpath='{.status.assessment.conclusions[0]}'`
if [ "$conclusion" != "All success criteria were  met" ]; then
  echo "Experiment failed unexpectedly!"
  exit 1
fi
echo "Experiment succeeded as expected!"

header "Clean up"
kubectl -n bookinfo-iter8 delete deployment productpage-v1
sleep 1
kubectl wait --for=delete deployment/productpage-v1 --timeout=540s
echo "Deployment productpage-v1 deleted"
