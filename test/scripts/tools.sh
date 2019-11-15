#!/usr/bin/env bash

set -e

if [ -z "${KUBECTL_VERSION}" ]
then
    KUBECTL_VERSION=v1.11.0
fi

if [ -z "${KUSTOMIZE_VERSION}" ]
then
    KUSTOMIZE_VERSION=1.0.10
fi

echo "installing kubectl"
curl -LO https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl
chmod +x ./kubectl
sudo mv ./kubectl /usr/local/bin/

echo "installing kustomize"
curl -OL "https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64"
sudo chmod +x kustomize_${KUSTOMIZE_VERSION}_linux_amd64
sudo mv kustomize_${KUSTOMIZE_VERSION}_linux_amd64 /usr/local/bin/kustomize
kustomize version

echo "installing idt"
curl -sL https://ibm.biz/idt-installer | bash


