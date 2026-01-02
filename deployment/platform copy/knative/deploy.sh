#!/bin/sh

echo "Install knative-v1.13.1"

echo "Installing Knative custom resources..."
kubectl apply -f serving-crds.yaml

echo "Installing Knative Serving..."
sleep 10
kubectl apply -f serving-core.yaml

echo "Installing Istio..."
sleep 10
kubectl apply -l knative.dev/crd-install=true -f istio.yaml
kubectl apply -f istio.yaml
kubectl apply -f net-istio.yaml

echo "Fetch Istio ingress gateway IP"
kubectl --namespace istio-system get service istio-ingressgateway

echo "Installing Magic DNS..."
kubectl apply -f serving-default-domain.yaml
