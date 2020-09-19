#!/usr/bin/env bash

# Simple header for logging purposes.
function header() {
  local upper="$(echo $1 | tr a-z A-Z)"
  make_banner "=" "${upper}"
}

# Display a box banner.
# Parameters: $1 - character to use for the box.
#             $2 - banner message.
function make_banner() {
    local msg="$1$1$1$1 $2 $1$1$1$1"
    local border="${msg//[-0-9A-Za-z _.,\/()]/$1}"
    echo -e "${border}\n${msg}\n${border}"
}

function test_experiment_status() {
  local experiment="$1"
  local expected="$2"
  local actual=`kubectl -n $NAMESPACE get experiments.iter8.tools $experiment -o=jsonpath='{.status.message}'`

  echo "Testing experiment .status.message"
  echo "   expecting status message: $expected"
  echo "         got status message: $actual"

  if [[ "$actual" != *"$expected"* ]]; then
    echo "FAIL: Got unexpected .status.message"
    echo "Teminating test case"
    kubectl -n iter8 logs $(kubectl -n iter8 get po --selector=app=iter8-controller -o jsonpath='{.items[0].metadata.name}')
    exit 1
  else
    echo "PASS: Got expected .status.message"
  fi
}

function test_vs_percentages() {
  local vs="$1"
  local route="$2"
  local expected="$3"
  local actual=$(kubectl -n $NAMESPACE get virtualservice.networking.istio.io $vs -o jsonpath="{.spec.http[0].route[$route].weight}")

  echo "Testing virtualservice .spec.http[0].route[$route].weight"
  echo "   expecting weight: $expected"
  echo "         got weight: $actual"

  if (( $expected == 0 )) && [[ -z $actual ]]; then
    echo "PASS: Got expected traffic split"
  else
    if [[ -z $actual ]] || (( $actual != $expected )); then
      echo "FAIL: Got unexpected traffic split"
      echo "Terminating test case"
      exit 1
    else
      echo "PASS: Got expected traffic split"
    fi
  fi
}

function test_winner_found() {
  local experiment="$1"
  local expected="$2"
  local actual=$(kubectl -n $NAMESPACE get experiments.iter8.tools $experiment -o jsonpath='{.status.assessment.winner.winning_version_found}')

  echo "Testing experiments.status.assessment.winner.winning_version_found"
  echo "   expecting winning_version_found: $expected"
  echo "         got winning_version_found: $actual"

  if [[ "$expected" == "$actual" ]]; then
    echo "PASS: matched winning_version_found"
  else
    echo "FAIL: did not match winning_version_found"
    echo "Terminating test case"
    exit 1
  fi 
}

function test_current_best() {
  local experiment="$1"
  local expected="$2"
  local id=$(kubectl -n $NAMESPACE get experiments.iter8.tools $experiment -o jsonpath='{.status.assessment.winner.current_best_version}')

  if [[ "$id" == "baseline" ]]; then 
    actual=$(kubectl -n $NAMESPACE get experiments.iter8.tools $experiment -o jsonpath="{.status.assessment.baseline.name}")
  else
    actual=$(kubectl -n $NAMESPACE get experiments.iter8.tools $experiment -o jsonpath="{.status.assessment.candidates[?(@.id==\"$id\")].name}")
  fi

  echo "Testing experiments.status.assessment.winner.current_best_version"
  echo "   expecting current_best_version: $expected"
  echo "         got current_best_version: $actual"

  if [[ "$expected" == "$actual" ]]; then
    echo "PASS: matched current_best_version"
  else
    echo "FAIL: did not match current_best_version"
    echo "Terminating test case"
    exit 1
  fi 
}

function test_deployment() {
  local deployment="$1"
  local present="$2" # valid values are in [true,false]

  kubectl -n $NAMESPACE get deployment $deployment
  if (( $? == 0 )); then
    # deployment is present
    if [[ "$present" == "true" ]]; then
      echo "PASS: expected deployment $deployment present"
    else # "$present" == "false"
      echo "FAIL: expected deployment $deployment not present"
      exit 1
    fi
  else
    # deployment is not present
    if [[ "$present" == "true" ]]; then
      echo "FAIL: deployment $deployment present"
      exit 1
    else # "$present" == "false"
      echo "PASS: deployment $deployment not present"
    fi
  fi
}
