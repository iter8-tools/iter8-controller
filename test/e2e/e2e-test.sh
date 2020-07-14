#!/usr/bin/env bash
  
# This script calls each end-to-end scenario sequentially and verifies the
# result

set -x

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

$DIR/e2e-scenario-0a.sh
$DIR/e2e-scenario-0b.sh
$DIR/e2e-scenario-0c.sh
# $DIR/e2e-scenario-1.sh
# $DIR/e2e-scenario-2.sh
# $DIR/e2e-scenario-3.sh
# $DIR/e2e-scenario-4.sh
# $DIR/e2e-scenario-5.sh
