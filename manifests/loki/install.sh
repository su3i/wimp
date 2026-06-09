#!/bin/bash
set -e

NS=loki

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "Script is located in: $SCRIPT_DIR"

# Check if the namespace exists
if ! kubectl get namespace "$NS" > /dev/null 2>&1; then
  echo "Namespace '$NS' does not exist. Creating it..."
  kubectl create namespace "$NS"
fi

helm upgrade --install loki oci://ghcr.io/grafana-community/helm-charts/loki \
    --namespace "$NS" \
 -f "$SCRIPT_DIR/values.yaml" \
