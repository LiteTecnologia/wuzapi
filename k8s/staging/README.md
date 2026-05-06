# wuzapi — Liteti staging deploy (RKE2)

Manifests Kubernetes para rodar wuzapi self-hosted no cluster RKE2 on-prem da Liteti.

Este overlay é **Liteti-specific** e não faz parte do projeto upstream `asternic/wuzapi`. Vive somente neste fork.

## Topologia

```
namespace: wuzapi-staging

  ┌─────────────────────────────────┐
  │ wuzapi (Deployment, 1 replica)  │
  │  • image: ghcr.io/.../wuzapi    │
  │  • port 8080 (ClusterIP only)   │
  └─────────────┬───────────────────┘
                │ DB_HOST=wuzapi-postgres
                ▼
  ┌─────────────────────────────────┐
  │ wuzapi-postgres                 │
  │  (StatefulSet, postgres:16,     │
  │   PVC 10Gi)                     │
  └─────────────────────────────────┘
```

**Sem ingress.** Acesso interno via `wuzapi.wuzapi-staging.svc.cluster.local:8080`. Para admin (criar instâncias, escanear QR), usar `kubectl port-forward`.

## Pré-requisitos

- RKE2 com kubeconfig em `~/.kube/rke2-liteti.yaml`
- StorageClass default disponível (ou ajustar `volumeClaimTemplates.storageClassName` no `postgres-statefulset.yaml`)
- Imagem do wuzapi publicada em `ghcr.io/litetecnologia/wuzapi` (build via GitHub Action — TODO)

## Provisionar

### 1. Gerar e armazenar tokens

```bash
pass insert -e liteti/services/faldesk-staging/wuzapi-admin-token <<< "$(openssl rand -hex 32)"
pass insert -e liteti/services/faldesk-staging/wuzapi-encryption-key <<< "$(openssl rand -hex 32)"
pass insert -e liteti/services/faldesk-staging/wuzapi-postgres-password <<< "$(openssl rand -hex 24)"
```

### 2. Criar namespace e secret antes do kustomize

```bash
export KUBECONFIG=~/.kube/rke2-liteti.yaml

kubectl apply -f k8s/staging/namespace.yaml

kubectl -n wuzapi-staging create secret generic wuzapi-secrets \
  --from-literal=WUZAPI_ADMIN_TOKEN="$(pass show liteti/services/faldesk-staging/wuzapi-admin-token)" \
  --from-literal=WUZAPI_GLOBAL_ENCRYPTION_KEY="$(pass show liteti/services/faldesk-staging/wuzapi-encryption-key)" \
  --from-literal=DB_PASSWORD="$(pass show liteti/services/faldesk-staging/wuzapi-postgres-password)" \
  --dry-run=client -o yaml | kubectl apply -f -
```

### 3. Aplicar manifests

```bash
kubectl apply -k k8s/staging
```

### 4. Verificar pods

```bash
kubectl -n wuzapi-staging get pods -w
# wuzapi-postgres-0    1/1  Running
# wuzapi-xxxxx-xxxxx   1/1  Running
```

## Smoke test

### 4.1 Port-forward

```bash
kubectl -n wuzapi-staging port-forward svc/wuzapi 8080:8080
```

### 4.2 Criar usuário (instância)

```bash
ADMIN_TOKEN="$(pass show liteti/services/faldesk-staging/wuzapi-admin-token)"

curl -X POST http://localhost:8080/admin/users \
  -H "Authorization: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"smoketest","token":"smoketest-token-123","jid":"","webhook":"","events":"All"}'
```

### 4.3 Conectar e pegar QR

```bash
curl -X POST http://localhost:8080/session/connect \
  -H "Token: smoketest-token-123" \
  -H "Content-Type: application/json" \
  -d '{"Subscribe":["Message"], "Immediate":true}'

curl -s http://localhost:8080/session/qr -H "Token: smoketest-token-123" \
  | jq -r '.data.QRCode' | qrencode -t ANSIUTF8
```

Se o QR aparecer no terminal e for escaneável → **wuzapi está OK em staging.**

## Build da imagem

Hoje a imagem é construída manualmente. Próximos passos (TODO):

- [ ] `.github/workflows/build-image.yml` — build em push pra `liteti/k8s-staging` e push pro `ghcr.io/litetecnologia/wuzapi:<sha>`
- [ ] Configurar pull secret no namespace ou tornar imagem pública

Para build manual:

```bash
docker build -t ghcr.io/litetecnologia/wuzapi:staging-$(git rev-parse --short HEAD) .
docker push ghcr.io/litetecnologia/wuzapi:staging-$(git rev-parse --short HEAD)
# Atualizar wuzapi-deployment.yaml com a tag
kubectl -n wuzapi-staging set image deployment/wuzapi wuzapi=ghcr.io/litetecnologia/wuzapi:staging-XXX
```

## Limpar tudo (se precisar reiniciar do zero)

```bash
kubectl delete namespace wuzapi-staging
# WAIT: isso apaga o PVC do postgres → todas as sessões WhatsApp são perdidas
```
