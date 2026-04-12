#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-kcore-crossplane}"
CROSSPLANE_NS="${CROSSPLANE_NS:-crossplane-system}"
ARGO_NS="${ARGO_NS:-argocd}"

log() { echo "[bootstrap] $*"; }

if ! command -v kind >/dev/null 2>&1; then
  echo "kind not found; use 'nix develop' in the repo root" >&2
  exit 1
fi
if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl not found" >&2
  exit 1
fi
if ! command -v helm >/dev/null 2>&1; then
  echo "helm not found" >&2
  exit 1
fi

if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  log "kind cluster ${CLUSTER_NAME} already exists"
else
  log "creating kind cluster ${CLUSTER_NAME}"
  kind create cluster --name "${CLUSTER_NAME}"
fi

kubectl get ns "${CROSSPLANE_NS}" >/dev/null 2>&1 || kubectl create ns "${CROSSPLANE_NS}"

if ! helm repo list 2>/dev/null | awk '{print $1}' | grep -qx crossplane-stable; then
  log "adding crossplane helm repo"
  helm repo add crossplane-stable https://charts.crossplane.io/stable
  helm repo update
fi

if ! helm status crossplane -n "${CROSSPLANE_NS}" >/dev/null 2>&1; then
  log "installing Crossplane"
  helm install crossplane crossplane-stable/crossplane -n "${CROSSPLANE_NS}" --wait --timeout 10m
else
  log "Crossplane release already installed"
fi

kubectl get ns "${ARGO_NS}" >/dev/null 2>&1 || kubectl create ns "${ARGO_NS}"

if ! kubectl get deploy argocd-server -n "${ARGO_NS}" >/dev/null 2>&1; then
  log "installing Argo CD (manifests)"
  kubectl apply -n "${ARGO_NS}" -f "https://raw.githubusercontent.com/argoproj/argo-cd/v2.14.5/manifests/install.yaml"
  log "waiting for argocd-server"
  kubectl rollout status deploy/argocd-server -n "${ARGO_NS}" --timeout=10m
else
  log "Argo CD already present in ${ARGO_NS}"
fi

log "done. kubectl context should point at kind-${CLUSTER_NAME}"
log "apply CRDs: kubectl apply -f package/crds/"
log "load provider image: docker build -t provider-kcore:dev . && kind load docker-image provider-kcore:dev --name ${CLUSTER_NAME}"
