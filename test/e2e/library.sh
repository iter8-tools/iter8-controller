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
    exit 1
  else
    echo "PASS: Got expected .status.message"
  fi
}
