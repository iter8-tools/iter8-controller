#!/usr/bin/env bash

set -e

if [ -z "${INSTALL_MINIKUBE}" ]; then
    exit 0
fi

MINIKUBE_VERSION=${MINIKUBE_VERSION:-v0.30.0}
BOOTSTRAPPER=${BOOTSTRAPPER:-kubeadm}
KUBE_VERSION=${KUBE_VERSION:-v1.12.0}

export MINIKUBE_WANTUPDATENOTIFICATION=false
export MINIKUBE_WANTREPORTERRORPROMPT=false
export CHANGE_MINIKUBE_NONE_USER=true
export MINIKUBE_HOME=$HOME

echo "installing nsenter"
if ! which nsenter; then
    curl -L https://github.com/minrk/git-crypt-bin/releases/download/trusty/nsenter > nsenter
    chmod +x nsenter
    sudo mv nsenter /usr/local/bin/
fi

# this is needed for kube > 1.11
echo "installing crictl"
curl -OL https://github.com/kubernetes-sigs/cri-tools/releases/download/${KUBE_VERSION}/crictl-${KUBE_VERSION}-linux-amd64.tar.gz
sudo tar zxvf crictl-${KUBE_VERSION}-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-${KUBE_VERSION}-linux-amd64.tar.gz

echo "installing minikube"
curl -Lo minikube https://storage.googleapis.com/minikube/releases/${MINIKUBE_VERSION}/minikube-linux-amd64
chmod +x minikube
sudo mv minikube /usr/local/bin/

echo "starting minikube"
export KUBECONFIG=$HOME/.kube/config

sudo -E minikube start --memory=8192 --cpus=4 --vm-driver=none --bootstrapper=${BOOTSTRAPPER} \
  --disk-size=30g \
  --extra-config=apiserver.enable-admission-plugins="LimitRanger,NamespaceExists,NamespaceLifecycle,ResourceQuota,ServiceAccount,DefaultStorageClass,MutatingAdmissionWebhook"\
  --kubernetes-version ${KUBE_VERSION}
echo "update context"
# minikube update-context

echo "waiting minikube to be ready"
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}';

until kubectl get nodes -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True";
do
  sleep 1;
done
