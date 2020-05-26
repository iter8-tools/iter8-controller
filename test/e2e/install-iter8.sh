#!/bin/sh
  
# Exit on error
set -e

# Build a new Iter8-controller image based on the new code
IMG=iter8-controller:test make docker-build

# Install Helm
curl -fsSL https://get.helm.sh/helm-v2.16.7-linux-amd64.tar.gz | tar xvzf - && sudo mv linux-amd64/helm /usr/local/bin

# Create new Helm template based on the new image
helm template install/helm/iter8-controller/ --name iter8-controller \
--set image.repository=iter8-controller \
--set image.tag=test \
--set image.pullPolicy=IfNotPresent \
> install/iter8-controller.yaml

cat install/iter8-controller.yaml

# Install Iter8-controller
kubectl apply -f install/iter8-controller.yaml

# Install Iter8 analytics
kubectl apply -f https://github.com/iter8-tools/iter8-analytics/releases/latest/download/iter8-analytics.yaml

# Check if Iter8 pods are all up and running. However, sometimes
# `kubectl apply` doesn't register for `kubectl wait` before, so
# adding 1 sec wait time for the operation to fully register
sleep 1
kubectl wait --for=condition=Ready pods --all -n iter8 --timeout=300s
kubectl -n iter8 get pods
