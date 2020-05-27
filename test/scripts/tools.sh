#!/usr/bin/env bash
  
set -e

if [ -z "${KUBECTL_VERSION}" ]
then
    KUBECTL_VERSION=v1.11.0
fi

echo "installing kubectl"
curl -LO https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl
chmod +x ./kubectl
sudo mv ./kubectl /usr/local/bin/
