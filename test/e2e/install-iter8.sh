#!/usr/bin/env bash
  
# Exit on error
set -e

CRD_VERSION=v1alpha2
ISTIO_NAMESPACE=istio-system

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"
source "$DIR/library.sh"

# Build a new Iter8-controller image based on the new code
header "build iter8-controller image"
IMG=iter8-controller:test make docker-build

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


if [ "$MIXER_DISABLED" = "false" ]; then
  # Create new Helm template based on the new image
  helm template install/helm/iter8-controller/ --name iter8-controller \
  --set image.repository=iter8-controller \
  --set image.tag=test \
  --set image.pullPolicy=IfNotPresent \
  -x templates/default/namespace.yaml \
  -x templates/default/manager.yaml \
  -x templates/default/serviceaccount.yaml \
  -x templates/crds/${CRD_VERSION}/iter8.tools_experiments.yaml \
  -x templates/metrics/iter8_metrics.yaml \
  -x templates/notifier/iter8_notifiers.yaml \
  -x templates/rbac/role.yaml \
  -x templates/rbac/role_binding.yaml \
  > install/iter8-controller.yaml
else
  echo "Using Istio telemetry v2"
  # Create new Helm template based on the new image
  helm template install/helm/iter8-controller/ --name iter8-controller \
  --set image.repository=iter8-controller \
  --set image.tag=test \
  --set image.pullPolicy=IfNotPresent \
  --set istioTelemetry=v2 \
  -x templates/default/namespace.yaml \
  -x templates/default/manager.yaml \
  -x templates/default/serviceaccount.yaml \
  -x templates/crds/${CRD_VERSION}/iter8.tools_experiments.yaml \
  -x templates/metrics/iter8_metrics.yaml \
  -x templates/notifier/iter8_notifiers.yaml \
  -x templates/rbac/role.yaml \
  -x templates/rbac/role_binding.yaml \
  > install/iter8-controller.yaml
fi

cat install/iter8-controller.yaml

# Install Iter8-controller
header "install iter8-controller"
kubectl apply -f install/iter8-controller.yaml

# Install Iter8 analytics
header "install iter8-analytics"
kubectl apply -f https://raw.githubusercontent.com/iter8-tools/iter8-analytics/v1.0.0-rc1/install/kubernetes/iter8-analytics.yaml

# Check if Iter8 pods are all up and running. However, sometimes
# `kubectl apply` doesn't register for `kubectl wait` before, so
# adding 1 sec wait time for the operation to fully register
sleep 1
kubectl wait --for=condition=Ready pods --all -n iter8 --timeout=300s
kubectl -n iter8 get pods

header "show iter8-analytics log"
kubectl -n iter8 logs `kubectl -n iter8 get pods | grep analytics | awk '{print $1}'`

header "show iter8-controller log"
kubectl -n iter8 logs `kubectl -n iter8 get pods | grep controller | awk '{print $1}'`
