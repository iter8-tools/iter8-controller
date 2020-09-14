#!/usr/bin/env bash

MINIKUBE_VERSION=v1.11.0
MINIKUBE_WANTUPDATENOTIFICATION=false
MINIKUBE_WANTREPORTERRORPROMPT=false
MINIKUBE_HOME=$HOME
CHANGE_MINIKUBE_NONE_USER=true
KUBECONFIG=$HOME/.kube/config

# Download kubectl
curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/$KUBE_VERSION/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/

# Install Helm
echo "install helm"
curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash

# Download minikube
curl -Lo minikube https://storage.googleapis.com/minikube/releases/$MINIKUBE_VERSION/minikube-linux-amd64 && chmod +x minikube && sudo mv minikube /usr/local/bin/

# Install conntrack (which seems necessary for k8s 1.18+)
# More details here: https://stackoverflow.com/questions/61238136/cant-start-minikube-in-ec2-shows-x-sorry-kubernetes-v1-18-0-requires-conntrac
sudo apt-get update
sudo apt install conntrack

# Create kube and minikube configuration directories
mkdir -p $HOME/.kube $HOME/.minikube
touch $KUBECONFIG
sudo minikube start --profile=minikube --vm-driver=none --kubernetes-version=$KUBE_VERSION
minikube update-context --profile=minikube
sudo chown -R travis: /home/travis/.minikube/
eval "$(minikube docker-env --profile=minikube)" && export DOCKER_CLI='docker'
