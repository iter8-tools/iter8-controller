## Instructions for setting up productpage v1 and v2 with reward metrics for Kubecon Demo

### Prelimnary:
1. Tested with Istio v1.6.3 and telemetry v2
2. Tested with Kubernetes v1.17
3. Apply `bookinfo-iter8` configured to enable auto-injection of the Istio sidecar according to the instructions [here](https://github.com/iter8-tools/docs/blob/v0.2.1/doc_files/iter8_bookinfo_istio.md#1-deploy-the-bookinfo-application)
4. Apply v1 of all services and deployments in bookinfo by running `kubectl apply -n bookinfo-iter8 -f bookinfo-tutorial.yaml`
4. Apply the gateway to be able to curl the application by running: `kubectl apply -n bookinfo-iter8 -f bookinfo-gateway.yaml`
4. Curl the application and check for a 200 response

### Start experiment:
1. Apply the experiment CRD to run an iter8 experiment between productpage v1 and productpage v2
2. Apply deployment and service spec for productoage-v2 using: `kubectl apply -n bookinfo-iter8 -f productpage-v2.yaml`
3. Apply gateway and VS for productpage-v2 using `kubectl apply -n bookinfo-iter8 -f productpage-gateway.yaml`
4. Curl the second version of productpage with the new host
5. Add target endpoint to Prometheus for `productpage-v1` and `productpage-v2` and restart prometheus pod. To do so, type `kubectl edit configmap -n istio-system prometheus`. In the editable yaml that appears, add the following target endpoints:

```
#Scrape custom metrics
- job_name: 'custom_metrics'
   static_configs:
   - targets: ['productpage.bookinfo-iter8.svc.cluster.local:9080', 'productpage.bookinfo-iter8.svc.cluster.local:9081']
```

6. Restart the prometheus pod so that the changes are reflected in Prometheus
6. Check if both target endpoints for custom metrics has the Status `UP` on the Prometheus UI.