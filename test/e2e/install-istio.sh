#!/usr/bin/env bash

# Relies on .travis.yml to set up environment variables

# Exit on error
#set -e

# From https://gist.githubusercontent.com/Ariel-Rodriguez/9e3c2163f4644d7a389759b224bfe7f3/raw/816453441da56058b73af5812db6217fb71897d2/semver.sh

# returns 1 when A greater than B
# returns 0 when A equals B
# returns -1 when A lower than B
#
# Author Ariel Rodriguez
# License MIT
###
semver_compare() {
  local version_a version_b pr_a pr_b
  # strip word "v" and extract first subset version (x.y.z from x.y.z-foo.n)
  version_a=$(echo "${1//v/}" | awk -F'-' '{print $1}')
  version_b=$(echo "${2//v/}" | awk -F'-' '{print $1}')

  if [ "$version_a" \= "$version_b" ]
  then
    # check for pre-release
    # extract pre-release (-foo.n from x.y.z-foo.n)
    pr_a=$(echo "$1" | awk -F'-' '{print $2}')
    pr_b=$(echo "$2" | awk -F'-' '{print $2}')

    ####
    # Return 0 when A is equal to B
    [ "$pr_a" \= "$pr_b" ] && echo 0 && return 0

    ####
    # Return 1

    # Case when A is not pre-release
    if [ -z "$pr_a" ]
    then
      echo 1 && return 0
    fi

    ####
    # Case when pre-release A exists and is greater than B's pre-release

    # extract numbers -rc.x --> x
    number_a=$(echo ${pr_a//[!0-9]/})
    number_b=$(echo ${pr_b//[!0-9]/})
    [ -z "${number_a}" ] && number_a=0
    [ -z "${number_b}" ] && number_b=0

    [ "$pr_a" \> "$pr_b" ] && [ -n "$pr_b" ] && [ "$number_a" -gt "$number_b" ] && echo 1 && return 0

    ####
    # Retrun -1 when A is lower than B
    echo -1 && return 0
  fi
  arr_version_a=(${version_a//./ })
  arr_version_b=(${version_b//./ })
  cursor=0
  # Iterate arrays from left to right and find the first difference
  while [ "$([ "${arr_version_a[$cursor]}" -eq "${arr_version_b[$cursor]}" ] && [ $cursor -lt ${#arr_version_a[@]} ] && echo true)" == true ]
  do
    cursor=$((cursor+1))
  done
  [ "${arr_version_a[$cursor]}" -gt "${arr_version_b[$cursor]}" ] && echo 1 || echo -1
}

# Install Istio
curl -L https://istio.io/downloadIstio | ISTIO_VERSION=${ISTIO_VERSION} sh -
istio-${ISTIO_VERSION}/bin/istioctl version
# Disable Kiali and grafana since not needed
if (( -1 == "$(semver_compare ${ISTIO_VERSION} 1.7.0)" )); then
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
