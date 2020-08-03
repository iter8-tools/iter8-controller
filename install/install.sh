#!/bin/bash

#set -x

# Use default istio namespace unless ISTIO_NAMESPACE is defined
: "${ISTIO_NAMESPACE:=istio-system}"

install() {

  echo "Istio namespace: $ISTIO_NAMESPACE"
  MIXER_DISABLED=`kubectl -n $ISTIO_NAMESPACE get cm istio -o json | jq .data.mesh | grep -o 'disableMixerHttpReports: [A-Za-z]\+' | cut -d ' ' -f2`
  ISTIO_VERSION=`kubectl -n $ISTIO_NAMESPACE get pods -o yaml | grep "image:" | grep proxy | head -n 1 | awk -F: '{print $3}'`

  if [ -z "$ISTIO_VERSION" ]; then
    echo "Cannot detect Istio version, aborting..."
    return
  elif [ -z "$MIXER_DISABLED" ]; then
    echo "Cannot detect Istio telemetry version, aborting..."
    return
  fi

  echo "Istio version: $ISTIO_VERSION"
  echo "Istio mixer disabled: $MIXER_DISABLED"

  if [ "$MIXER_DISABLED" = "false" ]; then
    echo "Using Istio telemetry v1"
    kubectl apply -f https://raw.githubusercontent.com/iter8-tools/iter8-controller/v1.0.0-preview/install/iter8-controller.yaml
  else
    echo "Using Istio telemetry v2"
	kubectl apply -f https://raw.githubusercontent.com/iter8-tools/iter8-controller/v1.0.0-preview/install/iter8-controller-telemetry-v2.yaml
  fi
  kubectl apply -f https://raw.githubusercontent.com/iter8-tools/iter8-analytics/v1.0.0-preview/install/kubernetes/iter8-analytics.yaml
}

install
