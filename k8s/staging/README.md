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
                │ DB_HOST=192.168.110.210
                ▼
  ┌─────────────────────────────────┐
  │ Postgres compartilhado Liteti   │
  │ (fora do cluster, gerenciado    │
  │  separadamente)                 │
  │  • database: wuzapi             │
  │  • user:     wuzapi             │
  └─────────────────────────────────┘
```

**Sem ingress.** Acesso interno via `wuzapi.wuzapi-staging.svc.cluster.local:8080`. Para admin (criar instâncias, escanear QR), usar `kubectl port-forward`.

**Postgres externo:** decidimos NÃO rodar postgres no cluster. Reaproveitamos a instância staging compartilhada da Liteti em `192.168.110.210:5432`. Vantagens: backup centralizado, menos consumo de Longhorn, banco sobrevive a `kubectl delete namespace`.

## Pré-requisitos

- RKE2 com kubeconfig em `~/.kube/rke2-liteti.yaml`
- Acesso de rede do cluster ao postgres compartilhado (`192.168.110.210:5432`)
- Imagem do wuzapi publicada em `ghcr.io/litetecnologia/wuzapi` (via workflow `liteti-build-image.yml`)

## Provisionar

### 1. Gerar e armazenar tokens

```bash
pass insert -e liteti/services/faldesk-staging/wuzapi-admin-token <<< "$(openssl rand -hex 32)"
pass insert -e liteti/services/faldesk-staging/wuzapi-encryption-key <<< "$(openssl rand -hex 32)"
pass insert -e liteti/services/faldesk-staging/wuzapi-postgres-password <<< "$(openssl rand -hex 24)"
```

### 2. Provisionar database e user no postgres compartilhado

Executar **uma única vez**, da rede `192.168.110.x` ou via SSH ao host compartilhado:

```bash
WUZAPI_PASS=$(pass show liteti/services/faldesk-staging/wuzapi-postgres-password)

PGPASSWORD=$(pass show liteti/postgres-admin-password) \
  psql -h 192.168.110.210 -U postgres <<SQL
CREATE DATABASE wuzapi;
CREATE USER wuzapi WITH ENCRYPTED PASSWORD '$WUZAPI_PASS';
GRANT ALL PRIVILEGES ON DATABASE wuzapi TO wuzapi;
\c wuzapi
GRANT ALL ON SCHEMA public TO wuzapi;
SQL
```

> wuzapi cria as tabelas próprias (whatsmeow sqlstore + tabelas locais) na primeira conexão. Não precisa rodar migrations manualmente.

### 3. Criar namespace e secret

```bash
export KUBECONFIG=~/.kube/rke2-liteti.yaml

kubectl apply -f k8s/staging/namespace.yaml

kubectl -n wuzapi-staging create secret generic wuzapi-secrets \
  --from-literal=WUZAPI_ADMIN_TOKEN="$(pass show liteti/services/faldesk-staging/wuzapi-admin-token)" \
  --from-literal=WUZAPI_GLOBAL_ENCRYPTION_KEY="$(pass show liteti/services/faldesk-staging/wuzapi-encryption-key)" \
  --from-literal=DB_PASSWORD="$(pass show liteti/services/faldesk-staging/wuzapi-postgres-password)" \
  --dry-run=client -o yaml | kubectl apply -f -
```

### 4. Aplicar manifests

```bash
kubectl apply -k k8s/staging
```

### 5. Verificar pods

```bash
kubectl -n wuzapi-staging get pods -w
# wuzapi-xxxxx-xxxxx   1/1  Running
```

Se o pod ficar em `Init:0/1`, é o init container `wait-for-postgres` esperando o postgres ficar acessível. Verifique:
- `kubectl -n wuzapi-staging logs <pod> -c wait-for-postgres`
- Conectividade do cluster ao `192.168.110.210:5432` (firewall, NetworkPolicies)

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

## Media storage — GCS via S3 interop

Bucket de mídia: `gs://nexodin-liteti-wuzapi-staging-media` (region `southamerica-east1`, UBLA, lifecycle delete 30 dias).

Service account: `wuzapi-staging-media@nexodin-liteti.iam.gserviceaccount.com` com `roles/storage.objectAdmin` apenas no bucket. HMAC keys salvas em `pass`:
- `liteti/services/wuzapi-staging/gcs-hmac-access-id`
- `liteti/services/wuzapi-staging/gcs-hmac-secret`

Configurar um user pra usar GCS via `POST /session/s3/config`:

```bash
ACCESS_ID=$(pass show liteti/services/wuzapi-staging/gcs-hmac-access-id)
SECRET=$(pass show liteti/services/wuzapi-staging/gcs-hmac-secret)
USER_TOKEN=$(pass show liteti/services/wuzapi-staging/user-caio-token)

curl -s -X POST https://wuzapi-staging.liteti.com.br/session/s3/config \
  -H "token: $USER_TOKEN" -H "Content-Type: application/json" \
  -d @- <<EOF
{
  "enabled": true,
  "endpoint": "https://storage.googleapis.com",
  "region": "auto",
  "bucket": "nexodin-liteti-wuzapi-staging-media",
  "access_key": "$ACCESS_ID",
  "secret_key": "$SECRET",
  "path_style": true,
  "public_url": "https://storage.googleapis.com/nexodin-liteti-wuzapi-staging-media",
  "media_delivery": "s3"
}
EOF
```

Provisão completa do bucket + SA + HMAC keys (one-shot, idempotente):

```bash
PROJECT=nexodin-liteti
BUCKET=nexodin-liteti-wuzapi-staging-media
REGION=southamerica-east1
SA=wuzapi-staging-media@$PROJECT.iam.gserviceaccount.com

gcloud storage buckets create gs://$BUCKET --project=$PROJECT --location=$REGION --uniform-bucket-level-access
gcloud iam service-accounts create wuzapi-staging-media --project=$PROJECT
gcloud storage buckets add-iam-policy-binding gs://$BUCKET --member="serviceAccount:$SA" --role="roles/storage.objectAdmin"
gcloud storage hmac create $SA --project=$PROJECT --format=json
# salvar accessId e secret em pass
```

**Caveat conhecido:** wuzapi usa `DeleteObjects` (batch) em `s3manager.go:405,425` ao executar `DELETE /admin/users/{id}/full`. GCS XML API não suporta batch delete. Cleanup massivo de mídia de um user via API vai falhar — a retenção fica delegada à lifecycle rule do bucket (30 dias, já configurada).

## Limpar tudo (se precisar reiniciar do zero)

```bash
# Cluster — não toca no postgres compartilhado
kubectl delete namespace wuzapi-staging

# Postgres — para reset completo das sessões WhatsApp
PGPASSWORD=$(pass show liteti/postgres-admin-password) \
  psql -h 192.168.110.210 -U postgres -c "DROP DATABASE wuzapi; CREATE DATABASE wuzapi; GRANT ALL PRIVILEGES ON DATABASE wuzapi TO wuzapi;"
```
