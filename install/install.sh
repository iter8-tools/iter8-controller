#!/bin/bash

# Install iter8 in one line; both the controller and the  analytics engine.
#set -x

# Use default istio namespace unless ISTIO_NAMESPACE is defined
: "${ISTIO_NAMESPACE:=istio-system}"

# Copied this here from hack/semver.sh since this is typically
# called via a curl pipe to shell
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

  # Install iter8 controller
  # Use the default YAML files created using make build-default
  # Three versions are created because the Prometheus queries needed are different for different
  # versions of Istio. The first change  took place with the removal of the mixer. The second in
  # version 1.7.0
  if [ "$MIXER_DISABLED" = "false" ]; then
    echo "Using Istio telemetry v1"
    kubectl apply -f https://raw.githubusercontent.com/iter8-tools/iter8-istio/master/install/iter8-controller.yaml
  else
    echo "Using Istio telemetry v2"
    if (( -1 == "$(semver_compare ${ISTIO_VERSION} 1.7.0)" )); then
	    kubectl apply -f https://raw.githubusercontent.com/iter8-tools/iter8-istio/master/install/iter8-controller-telemetry-v2.yaml
    else
    	kubectl apply -f https://raw.githubusercontent.com/iter8-tools/iter8-istio/master/install/iter8-controller-telemetry-v2-17.yaml
    fi
  fi

  # Install  iter8 analytics
  kubectl apply -f https://raw.githubusercontent.com/iter8-tools/iter8-analytics/master/install/kubernetes/iter8-analytics.yaml
}

install
