## Instructions for setting up productpage v1 and v2 with reward metrics for Kubecon Demo

### Prelimnary:
1. Tested with Istio v1.6.3 and telemetry v2
2. Tested with Kubernetes v1.17
3. Comment out TLS congurations for in Istio's prometheus configmap. To do so, type `kubectl edit configmap -n istio-system prometheus`. In the editable yaml that appears, comment out the some lines:

```
- job_name: 'kubernetes-pods'
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:  # If first two labels are present, pod should be scraped  by the istio-secure job.
  - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
    action: keep
    regex: true
  #- source_labels: [__meta_kubernetes_pod_annotation_sidecar_istio_io_status]
  #  action: drop
  #  regex: (.+)
  #- source_labels: [__meta_kubernetes_pod_annotation_istio_mtls]
  #  action: drop
  #  regex: (true)
  - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
    action: replace
    target_label: __metrics_path__
    regex: (.+)
  - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
    action: replace
    regex: ([^:]+)(?::\d+)?;(\d+)
    replacement: $1:$2
    target_label: __address__
```
3. Apply `bookinfo-iter8` configured to enable auto-injection of the Istio sidecar according to the instructions [here](https://github.com/iter8-tools/docs/blob/v0.2.1/doc_files/iter8_bookinfo_istio.md#1-deploy-the-bookinfo-application)
4. Apply v1 of all services and deployments in bookinfo by running `kubectl apply -n bookinfo-iter8 -f kc-bookinfo-tutorial.yaml`
4. Apply the gateway to be able to curl the application by running: `kubectl apply -n bookinfo-iter8 -f kc-bookinfo-gateway.yaml`
4. Curl the application and check for a 200 response

### Start experiment:
1. Apply the experiment CRD to run an iter8 experiment between productpage v1 and productpage v2
2. Apply deployment and service spec for productoage-v2 using: `kubectl apply -n bookinfo-iter8 -f kc-productpage-v2.yaml`
3. Apply deployment and service spec for productoage-v3 using: `kubectl apply -n bookinfo-iter8 -f kc-productpage-v3.yaml`
4. Curl v2 and v3 of productpage with the new host and check for a 200 response
5. Check if both target endpoints for custom metrics has the Status `UP` on the Prometheus UI.
