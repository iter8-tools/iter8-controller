#!/usr/bin/env bash
  
# This script calls each end-to-end scenario sequentially and verifies the
# result

DIR="$( cd "$( dirname "$0" )" >/dev/null 2>&1; pwd -P )"

# install yq
which yq
if (( $? )); then
  sudo apt-get update
  sudo apt-get install software-properties-common
  sudo add-apt-repository -y ppa:rmescandon/yq
  sudo apt update
  sudo apt install yq -y
fi

# Exit on error
set -e

$DIR/e2e-canary-scenario-1.sh
$DIR/e2e-canary-scenario-2.sh
# later versions do not require prometheus config changes
if [ "-1" != $(${DIR}/../../hack/semver.sh ${ISTIO_VERSION} 1.7.0) ]; then
  $DIR/e2e-abn-scenario-1.sh
fi