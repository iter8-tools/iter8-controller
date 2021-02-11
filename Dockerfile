# Build the manager binary
FROM golang:1.14.1 as builder

# Copy in the go src
WORKDIR /go/src/github.com/iter8-tools/iter8-istio
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/iter8-tools/iter8-istio/cmd/manager

# Copy the controller-manager into a thin image
FROM ubuntu:latest

RUN apt-get update
RUN apt-get install -y ca-certificates

WORKDIR /
COPY --from=builder /go/src/github.com/iter8-tools/iter8-istio/manager .
ENTRYPOINT ["/manager"]
