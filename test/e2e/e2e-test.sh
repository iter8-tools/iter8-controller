#!/usr/bin/env bash
  
# This script calls each end-to-end scenario sequentially and verifies the
# result

# Exit on error
set -e

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"

$DIR/../../iter8-trend/test/e2e-scenario-1.sh
$DIR/../../iter8-trend/test/e2e-scenario-2.sh
$DIR/../../iter8-trend/test/e2e-scenario-3.sh
$DIR/../../iter8-trend/test/e2e-scenario-4.sh
$DIR/../../iter8-trend/test/e2e-scenario-5.sh

# Do some cleanups
kubectl delete -f install/iter8-controller.yaml
kubectl delete -f https://github.com/iter8-tools/iter8-analytics/releases/latest/download/iter8-analytics.yaml
kubectl delete ns iter8
