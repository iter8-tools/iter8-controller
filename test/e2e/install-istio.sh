#!/usr/bin/env bash

# Relies on .travis.yml to set up environment variables

# Exit on error
set -e

# Install Istio
curl -L https://istio.io/downloadIstio | ISTIO_VERSION=${ISTIO_VERSION} sh -
istio-${ISTIO_VERSION}/bin/istioctl version
# Disable Kiali as it sometimes does not come up
istio-${ISTIO_VERSION}/bin/istioctl manifest apply --set profile=demo \
--set values.kiali.enabled=false --set values.grafana.enabled=false
sleep 1
kubectl wait --for=condition=Ready pods --all -n istio-system --timeout=600s
kubectl -n istio-system get pods
