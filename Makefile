# Image URL to use all building/pushing image targets
IMG ?= iter8-controller:latest

all: manager

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/iter8-tools/iter8-controller/cmd/manager

# Run against the Kubernetes cluster configured in $KUBECONFIG or ~/.kube/config
run: generate fmt vet load
	go run ./cmd/manager/main.go

# Generate iter8 crds and rbac manifests
manifest:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd \
	  --output-dir install/helm/iter8-controller/templates/crds
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go rbac \
	  --output-dir install/helm/iter8-controller/templates/rbac \
	  --service-account controller-manager \
	  --service-account-namespace "{{ .Values.namespace }}"
	./hack/crd_fix.sh
	sed -i -e "13s/\'//g" install/helm/iter8-controller/templates/rbac/manager_role_binding.yaml
	rm -f ./install/helm/iter8-controller/templates/rbac/manager_role_binding.yaml-e

# Prepare Kubernetes cluster for iter8 (running in cluster or locally):
#   install CRDs
#   install configmap/iter8-metrics is defined in namespace iter8 (creating namespace if needed)
load: manifest
	helm template install/helm/iter8-controller \
	  --name iter8-controller \
	  -x templates/default/namespace.yaml \
	  -x templates/crds/iter8_v1alpha1_experiment.yaml \
	  -x templates/metrics/iter8_metrics.yaml \
	| kubectl apply -f -

# Deploy controller to the Kubernetes cluster configured in $KUBECONFIG or ~/.kube/config
deploy: manifest
	helm template install/helm/iter8-controller \
	  --name iter8-controller \
	  --set image.repository=`echo ${IMG} | cut -f1 -d':'` \
	  --set image.tag=`echo ${IMG} | cut -f2 -d':'` \
	| kubectl apply -f -

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

build-default:
	echo '# Generated by make build-default; DO NOT EDIT' > install/iter8-controller.yaml
	helm template install/helm/iter8-controller \
   		--name iter8-controller \
	>> install/iter8-controller.yaml

tests:
	go test ./test/.
	test/e2e/e2e.sh --skip-setup