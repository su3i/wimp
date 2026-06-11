#!/bin/bash
set -e

NS=prometheus
RELEASE=prometheus-operator

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "Script is located in: $SCRIPT_DIR"

# Check if the namespace exists
if ! kubectl get namespace "$NS" > /dev/null 2>&1; then
  echo "Namespace '$NS' does not exist. Creating it..."
  kubectl create namespace "$NS"
fi

helm upgrade --install $RELEASE prometheus-community/kube-prometheus-stack \
  --namespace "$NS" \
  -f "$SCRIPT_DIR/values.yaml" \
  \
  --set alertmanager.enabled=true \
  --set grafana.enabled=true \
  \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues=false \
  \
  --set grafana.service.type=LoadBalancer \
  --set grafana.persistence.enabled=true \
  --set grafana.persistence.size=10Gi \
  \
  --set alertmanager.alertmanagerSpec.retention=48h \
  \
  --set kube-state-metrics.enabled=true \
  --set nodeExporter.enabled=true
