#!/usr/bin/env bash

# Exit on error
set -e

# Sanity check K8s cluster
kubectl version
kubectl cluster-info
