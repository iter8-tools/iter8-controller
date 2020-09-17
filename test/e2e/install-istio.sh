#!/usr/bin/env bash

# Relies on .travis.yml to set up environment variables

# Exit on error
#set -e

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"

# Install Istio

# Get specified version of Istio
curl -L https://istio.io/downloadIstio | ISTIO_VERSION=${ISTIO_VERSION} sh -
istio-${ISTIO_VERSION}/bin/istioctl version

# Disable Kiali and grafana since not needed
# Use different istioctl command depending on Istio version
if (( -1 == "$(${DIR}/../../hack/semver.sh ${ISTIO_VERSION} 1.7.0)" )); then
  istio-${ISTIO_VERSION}/bin/istioctl manifest apply \
    --set profile=demo \
    --set values.kiali.enabled=false \
    --set values.grafana.enabled=false
else
  istio-${ISTIO_VERSION}/bin/istioctl manifest install \
    --set profile=demo \
    --set values.kiali.enabled=false \
    --set values.grafana.enabled=false \
    --set values.prometheus.enabled=true
fi

# wait for pods to come up
sleep 1
kubectl wait --for=condition=Ready pods --all -n istio-system --timeout=540s
kubectl -n istio-system get pods
