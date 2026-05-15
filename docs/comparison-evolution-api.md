# Comparativo Técnico: wuzapi (Liteti fork) vs evolution-api

**Data:** 2026-05-15
**Método:** análise estática dos dois repos. Sem benchmarks de runtime.
**Repos auditados:**
- wuzapi: `/home/caio/liteti/code/liteti-wuzapi` (branch `liteti/k8s-staging`, fork de `github.com/asternic/wuzapi`)
- evolution-api: `/home/caio/liteti/code/evolution-api-liteti`

**Validação do fork vs upstream wuzapi (`asternic/wuzapi`):**
- O fork Liteti está **6 commits à frente** do upstream — todos infra (k8s, CI, Cloudflare Tunnel), **zero mudança em código Go**.
- O upstream está **6 commits à frente** do fork — 4 sobre workflow `updatecontrib.yml` / README / `contributors.html` (zero impacto funcional), 2 sobre o mesmo hardening: comparação do admin token via `sha256` + `subtle.ConstantTimeCompare` (10 linhas em `handlers.go:127`).
- **Conclusão:** o comparativo deste documento reflete o que o wuzapi oferece hoje. Atualizar o fork pra upstream HEAD adiciona apenas um fix de segurança trivial; não há features ou melhorias arquiteturais sub-representadas.

> Documento factual. Toda afirmação tem path/linha como evidência. Interpretações estão na seção [Análise crítica](#análise-crítica) e claramente separadas.

---

## Sumário executivo

| Dimensão | wuzapi | evolution-api | Vantagem |
|---|---|---|---|
| LoC backend | 13.754 Go | 32.675 TS | wuzapi (~42%) |
| Dependências runtime | 23 diretas / 57 transitivas | 60 NPM | wuzapi (~38%) |
| Endpoints REST | 71 | 65 | empate |
| WhatsApp lib | `whatsmeow` (Go, Beeper) | `baileys` 7.0-**rc.9** (Node) | depende* |
| Chatbot integrations embutidas | 0 | 8 (Chatwoot, Typebot, OpenAI, Dify, n8n, evoai, Flowise, evolutionBot) | evolution |
| Event brokers | 2 (webhook, RabbitMQ) | 7 (webhook, websocket, RabbitMQ, NATS, SQS, Kafka, Pusher) | evolution |
| DB providers | Postgres + SQLite | Postgres / MySQL / Mongo | evolution |
| Persistência de contatos | ✅ `whatsmeow_contacts` (lib) | ✅ `Contact` (Prisma) | empate |
| Persistência de chat settings | ✅ `whatsmeow_chat_settings` (lib) | ✅ `Chat` (Prisma) | empate |
| Histórico de mensagens | opcional via flag `history>0` (campos minimal) | nativo (campos completos + `MessageUpdate`) | evolution |
| Entidade Chat agregada (last msg, unread, name) | ❌ vem da agregação | ✅ model próprio | evolution |
| Prometheus `/metrics` | ❌ | ✅ | evolution |
| Logs estruturados JSON | ✅ (zerolog) | ❌ (logger custom ANSI) | wuzapi |
| Sentry/GlitchTip | precisa integrar | nativo | evolution |
| Multi-channel (Meta Cloud API) | ❌ | ✅ | evolution |
| Escala horizontal real | ❌ (stateful, replicas=1) | ❌ (stateful, in-memory map) | empate |
| Templates Meta | ❌ (handler existe, rota comentada em `routes.go:111`) | ✅ | evolution |

\* "Depende" = decisão depende do uso atual (ver [Análise crítica](#análise-crítica)).

---

## 1. Features & paridade API

### Mensageria — funcionalidades de envio

| Tipo | wuzapi | evolution-api | Notas |
|---|---|---|---|
| Texto | ✅ `/chat/send/text` (`handlers.go:2570`) | ✅ `sendText` | — |
| Imagem | ✅ `/chat/send/image` (`handlers.go:1182`) | ✅ `sendMedia` | — |
| Vídeo | ✅ `/chat/send/video` (`handlers.go:1553`) | ✅ `sendMedia` | evolution unifica em `sendMedia` |
| Áudio | ✅ `/chat/send/audio` (`handlers.go:989`) | ✅ `sendWhatsAppAudio` | — |
| Documento | ✅ `/chat/send/document` (`handlers.go:814`) | ✅ `sendMedia` | — |
| Sticker | ✅ `/chat/send/sticker` (`handlers.go:1385`) | ✅ `sendSticker` | — |
| Localização | ✅ `/chat/send/location` (`handlers.go:1849`) | ✅ `sendLocation` | — |
| Contato (vCard) | ✅ `/chat/send/contact` (`handlers.go:1727`) | ✅ `sendContact` | — |
| Botões | ✅ `/chat/send/buttons` (`handlers.go:1973`) | ✅ `sendButtons` | feature desativada pelo WA em 2023, ambos mantêm o endpoint |
| Lista | ✅ `/chat/send/list` (`handlers.go:2225`) | ✅ `sendList` | — |
| Enquete (poll) | ✅ `/chat/send/poll` (`handlers.go:2697`) | ✅ `sendPoll` | wuzapi tem decode plaintext de votos (`clients.go:18`) |
| Reação | ✅ `/chat/react` (`handlers.go:3877`) | ✅ `sendReaction` | — |
| Edição | ✅ `/chat/edit` (`handlers.go:2845`) | ✅ `updateMessage` | — |
| Status (story) | ✅ `/status/set/text` (`handlers.go:2517`) | ✅ `sendStatus` | evolution tem mais tipos de status (mídia, áudio) |
| PTV (video note) | ❌ | ✅ `sendPtv` | — |
| Template Meta Cloud API | ❌ rota comentada | ✅ `create/edit/find/delete` (`template.router.ts`) | — |

### Operações sobre chat/mensagem

| Operação | wuzapi | evolution-api |
|---|---|---|
| Deletar mensagem | ✅ `/chat/delete` (`handlers.go:2780`) | ✅ `deleteMessageForEveryone` |
| Marcar como lida | ✅ `/chat/markread` (`handlers.go:3984`) | ✅ `markMessageAsRead` |
| Arquivar chat | ✅ `/chat/archive` (`handlers.go:6825`) | ✅ `archiveChat` |
| Histórico de mensagens | ✅ `/chat/history` (`handlers.go:6229`) | ✅ `findMessages` (consulta o cache Prisma) |
| Marcar chat não-lido | ❌ | ✅ `markChatUnread` |
| Verificar se número tem WA | ✅ `/user/check` (`handlers.go:3216`+) | ✅ `whatsappNumbers` |
| Update/bloquear contato | parcial | ✅ `updateBlockStatus` |
| Privacidade (read receipts, last seen) | ❌ | ✅ `fetch/updatePrivacySettings` |

### Grupos

Ambos têm cobertura completa: criar, listar, info, alterar nome/foto/descrição, participantes (add/remove/promote/demote), invite link (gerar/revogar), aceitar invite, sair, alternar settings (announce/locked/ephemeral).

### Recursos avançados

| Feature | wuzapi | evolution-api |
|---|---|---|
| Chamadas (offer fake) | ❌ (só `/call/reject` em `handlers.go:6624`) | ✅ `POST /call/offer` |
| Catalog / Business profile | ❌ | ✅ `business.router.ts` (getCatalog, getCollections) |
| Labels do WA | ❌ | ✅ `findLabels`, `handleLabel` |
| Newsletter / Channels | ✅ `/newsletter/list` (`handlers.go:4967`) | parcial |
| Proxy por instância | ✅ `/session/proxy` (`handlers.go:5840`) | ✅ `proxy.router.ts` (set/find) |
| HMAC signing de webhooks | ✅ `/session/hmac/config` (`handlers.go:6499`) | ❌ |
| S3 por usuário | ✅ `/session/s3/*` (`handlers.go:5939`+) | ✅ global (não por instância) |
| Modo stdio JSON-RPC | ✅ `stdio.go` (629 LoC) | ❌ |

### Integrações chatbot / IA

| Plataforma | wuzapi | evolution-api |
|---|---|---|
| Chatwoot | ❌ | ✅ |
| Typebot | ❌ | ✅ |
| OpenAI assistant nativo | ❌ | ✅ |
| Dify | ❌ | ✅ |
| n8n | ❌ | ✅ |
| Flowise | ❌ | ✅ |
| evoai / evolutionBot | ❌ | ✅ |

> **wuzapi propõe um modelo de "REST puro"**: você integra com sua plataforma externamente. evolution-api inclui adapters embutidos.

### Canais (channels)

| Canal | wuzapi | evolution-api |
|---|---|---|
| WhatsApp Web (Baileys/whatsmeow) | ✅ | ✅ |
| **Meta Cloud API oficial** | ❌ | ✅ (`integrations/channel/meta/`) |
| Evolution channel (custom) | — | ✅ |

---

## 2. Estabilidade lib + Performance

### whatsmeow vs Baileys

| Aspecto | whatsmeow | Baileys |
|---|---|---|
| Linguagem | Go | TypeScript/Node |
| Mantenedor | Tulir Asokan (Beeper, comercial) | comunidade open-source (WhiskeySockets) |
| Versão em uso | pseudo-version pinned `v0.0.0-20260305...fc65416c22c4` (`go.mod:15`) | `7.0.0-rc.9` (`package.json:65`) — **release candidate, não stable** |
| Cadência de releases | nightly via Git SHA | release candidate atual; histórico de quebras grandes em majors (5→6→7) |
| Surface de features | menor (sem catalog real, sem call offer, templates limitados) | maior (call, catalog, status mídia, business) |
| Performance baseline (consensus comunidade) | mais lean, sem event loop overhead | bem otimizado pra Node, mas single-thread |

**Observação factual sobre o pinning:** wuzapi usa pseudo-version do Git (não release tagged), o que significa atualização exige rebuild do go.mod sempre. Evolution está em **release candidate** (não GA), o que é incomum em produção.

### Concorrência e modelo de execução

**wuzapi:**
- 1 goroutine "keep-alive" **busy-wait 1s** por sessão (`wmiau.go:646-666`). Para 100 sessões = 100 goroutines acordando a cada segundo.
- Goroutines adicionais sob demanda: webhook delivery (`wmiau.go:95`), history sync (`wmiau.go:749`), S3 init (`wmiau.go:302`).
- `ClientManager` global protegido por `sync.RWMutex` (`clients.go:10-22`).
- Caches in-memory: `userinfocache` 5min TTL, `lastMessageCache` 24h, `pollOptions` sem TTL.

**evolution-api:**
- Node single-thread, single-process. Sem cluster, PM2, worker_threads ou Bull (verificado em `package.json` / `src/main.ts`).
- `WaInstances: Record<string, WaInstance>` em memória (`monitor.service.ts:40`).
- Carregado via `waMonitor.loadInstance()` no boot (`main.ts:30`).

> Ambos são **fundamentalmente single-node, stateful**. Diferença: Go com goroutines escala melhor sob carga de CPU paralela; Node serializa tudo no event loop.

### Footprint e dependências

| | wuzapi | evolution-api |
|---|---|---|
| k8s requests | cpu=100m, mem=256Mi (`k8s/staging/wuzapi-deployment.yaml:69`) | cpu=50m, mem=256Mi (`k8s/base/deployment.yaml:183`) |
| k8s limits | cpu=1000m, mem=1Gi | cpu=500m, mem=512Mi |
| Base image runtime | `debian:bookworm-slim` + ffmpeg + pg-client + curl | `node:24-alpine` + ffmpeg |
| Body limit HTTP | (não setado explicitamente) | 136 MB (`src/main.ts:79`) |
| Reconexão WA | 3 tentativas, backoff linear 5s (`wmiau.go:394-614`) | auto-reconnect exceto `loggedOut/forbidden/402/406` |
| Deps pesadas | ffmpeg (runtime), CGO=1 por SQLite (`Dockerfile:21`) | sharp, jimp, ffmpeg, kafkajs, nats, @aws-sdk/sqs, minio, openai, redis, pg |

**Dependency surface** (risco supply chain):
- wuzapi: 23 diretas no `go.mod`, 57 incluindo indirect, 237 entradas no `go.sum`
- evolution-api: 60 prod + 27 dev = 87 NPM packages diretos; transitivamente NPM costuma multiplicar 10-30x

### Persistência de auth state (sessão WhatsApp)

| | wuzapi | evolution-api |
|---|---|---|
| Padrão | tabelas `whatsmeow_*` no mesmo Postgres (`sqlstore.Container` em `main.go:402`) | escolhível: filesystem `provider/sessions`, Redis, ou Postgres via Prisma model `Session` |
| Implicação k8s | volume não necessário — tudo no DB | filesystem padrão exige `PVC`; em staging usa `emptyDir` (perde sessão em restart) |

> **wuzapi tem vantagem operacional aqui:** zero volume, zero gestão de PVC, sessão sobrevive a qualquer recreate de pod.

---

## 3. Multi-tenant & Operação

### Modelo de tenant

**wuzapi — schema real no Postgres (19 tabelas, verificado em staging):**

- **App-level (3 tabelas):**
  - `users` (`migrations.go:106-117` + migrations 4-8): `id, name, token, webhook, jid, qrcode, connected, expiration, events, proxy_url, s3_*, media_delivery, s3_retention_days, history, hmac_key`. Flat — sem team/org.
  - `message_history` (opcional via flag `history>0` no user): `id, user_id, chat_jid, sender_jid, message_id, timestamp, message_type, text_content, media_link`. **Campos minimal — sem delivery status, sem recipients, sem tracking de edição/delete.**
  - `migrations` — controle de versão.

- **Lib-level (`whatsmeow_*`, 16 tabelas):** persistidas automaticamente pela lib whatsmeow no mesmo Postgres. Inclui:
  - `whatsmeow_contacts` (queryable via `GET /user/contacts` em `handlers.go:3472`): `our_jid, their_jid, first_name, full_name, push_name, business_name, redacted_phone`.
  - `whatsmeow_chat_settings`: `muted_until, pinned, archived` por chat.
  - `whatsmeow_device, sessions, identity_keys, pre_keys, sender_keys, app_state_*, message_secrets, lid_map, event_buffer, retry_buffer, privacy_tokens, version`.

> **Importante**: contatos e chat settings são persistidos by-default sem ação explícita do app — não precisa ativar a flag `history`. Só o histórico de **mensagens** requer opt-in.

**evolution-api — model `Instance` (Prisma)** (`prisma/postgresql-schema.prisma:60`):
- 36 models Prisma com FK `instanceId` em cada um (cascade delete).
- Persiste: `Chat`, `Contact`, `Message`, `MessageUpdate`, `Webhook`, `Chatwoot`, `Label`, `Proxy`, `Setting`, `Rabbitmq`, `Nats`, `Sqs`, `Kafka`, `Websocket`, `Pusher`, `Typebot`, `OpenaiBot`...
- Discriminator extra: `clientName` (env `CLIENT_NAME`) — permite múltiplos clusters de instâncias compartilhando o mesmo DB.
- 58 migrations Postgres versionadas.

> **Diferenças reais de persistência (verificado contra DB staging):**
> - Contatos e chat settings: ambos persistem. wuzapi via `whatsmeow_*` (lib), evolution via Prisma. Funcional paridade.
> - Histórico de mensagens: wuzapi tem como opt-in com campos minimal; evolution é nativo com metadata completa (`MessageUpdate` rastreia edits/deletes/status delivery).
> - Entidade "Chat" agregada (lista de conversas com last message, unread count, nome do chat): existe no evolution como model próprio; no wuzapi precisa ser agregada a partir de `message_history` + `whatsmeow_contacts`.

### Escala horizontal

**Ambos são stateful single-node por design.**

- wuzapi: `clients.go:18-22` documenta: "poll plaintext is in-memory only — if wuzapi restarts between send and vote, plaintext resolution is skipped". k8s manifest tem `replicas: 1` + `strategy: Recreate` com comentário explícito.
- evolution-api: `waInstances: Record<string, any>` em memória; reload exige restart. Sem socket.io Redis adapter pra pub/sub entre nós.

Pra **escalar horizontalmente**, ambos exigem sharding por instance/user na entrada (proxy reverso roteia request pro pod que tem a sessão) — nenhum oferece isso pronto.

### Observabilidade

| | wuzapi | evolution-api |
|---|---|---|
| Healthcheck | `/health` rico (uptime, active_connections, total_users, connected_users, logged_in_users, mem_stats, goroutines, version) — `handlers.go:46-119` | TCP socket probe apenas (`k8s/base/deployment.yaml:190-204`); Dockerfile usa `wget /` |
| Prometheus `/metrics` | ❌ não existe | ✅ gated por `PROMETHEUS_METRICS=true` (`index.router.ts:93-161`): `evolution_environment_info`, `evolution_instances_total`, `evolution_instance_up{instance,integration}`, `evolution_instance_state` |
| Logs estruturados | ✅ zerolog JSON via `--logtype=json` (`go.mod:13`) + middleware `hlog` | ❌ logger custom ANSI-colored; pino só em deps via Baileys |
| Sentry/GlitchTip | ❌ exige integração externa | ✅ `@sentry/node` via `instrumentSentry` em `main.ts:1`, gated por `SENTRY_DSN` |
| Grafana dashboard pronto | ❌ | ✅ `grafana-dashboard.json.example` no root |

### Webhook delivery

| | wuzapi | evolution-api |
|---|---|---|
| Disparo | goroutine-per-event (`wmiau.go:95`,`99`) | in-process |
| Retry | backoff exp `1<<(n-1) × 30s`, 5 tentativas, dead-letter pra RabbitMQ `webhook_errors` (`helpers.go:264-365`) | 10 tentativas, sem fila persistente (`webhook.controller.ts:213`) |
| HMAC | ✅ SHA-256 opcional por usuário (`helpers.go:248`) | ❌ |
| SSRF guard | ✅ bloqueia IPs privados (`main.go:85-145`) | ❌ explícito |

### Event brokers (alternativas ao webhook)

| Broker | wuzapi | evolution-api |
|---|---|---|
| Webhook HTTP | ✅ | ✅ |
| RabbitMQ | ✅ opcional (`RABBITMQ_URL`) — filas `whatsapp_events` + `webhook_errors` | ✅ opcional (`RABBITMQ_ENABLED`) |
| Websocket (servidor push pros clientes) | ❌ | ✅ `socket.io` |
| NATS | ❌ | ✅ |
| Kafka | ❌ | ✅ `kafkajs` |
| AWS SQS | ❌ | ✅ |
| Pusher | ❌ | ✅ |

---

## Métricas brutas

| | wuzapi | evolution-api |
|---|---|---|
| LoC backend | 13.754 Go (12.927 sem testes) | 32.675 TS |
| Arquivos backend | 14 .go | 187 .ts |
| Arquivo mais denso | `handlers.go` 7.127 LoC | (distribuído) |
| Endpoints REST | 71 | 65 (excl. integrations) |
| Handlers | 75 (`func (s *server) ...`) | ~equivalente, modular |
| Modelos de persistência | 3 app + 16 lib (whatsmeow_*) = **19 tabelas** | 36 modelos Prisma |
| Migrations app | 9 | 58 |
| Dependências diretas | 23 (go.mod) | 60 (package.json) |
| Tamanho `go.sum` / lockfile | 237 entradas | (não medido em deps tree) |
| Event types | 50 (`constants.go:4-76`) | comparable |
| Versão app | sem release tag (pseudo-version) | `2.3.7` (`package.json:3`) |

---

## Análise crítica

> A partir daqui há **interpretação**. Está separado explicitamente das tabelas acima, que são puramente factuais.

### Quando wuzapi ganha objetivamente

1. **Superfície de manutenção menor** — 42% LoC, 38% deps. Code review e auditoria de segurança proporcionalmente mais baratos.
2. **Operação simpler** — sem PVC pra session state, healthcheck rico nativo, logs JSON prontos pra Loki/GlitchTip.
3. **Segurança em webhooks** — HMAC SHA-256 + SSRF guard nativos. evolution-api precisa de WAF/sidecar pra mesmo nível.
4. **whatsmeow** — mantido comercialmente (Beeper), historicamente estável a mudanças do WA. Baileys 7.x ainda está em **release candidate**, o que é red flag pra produção.

### Quando evolution-api ganha objetivamente

1. **Cobertura de features** — Meta Cloud API, catalog, business profile, labels, templates, PTV, mais opções de status, privacy settings. **Se você usa qualquer um desses hoje, migrar pro wuzapi é regredir.**
2. **Metadata avançada de mensagens** — wuzapi persiste contatos (`whatsmeow_contacts`), chat settings e — se `history>0` — também mensagens em `message_history` com campos minimal (id, type, text, media_link). O que evolution tem que wuzapi não tem: `MessageUpdate` tracking (status de entrega, edits, deletes) e entidade `Chat` agregada nativa (nome do chat, last message, unread count). Se você consulta esses dados específicos hoje, precisa portar ou estender.
3. **Integrações chatbot embutidas** — 8 plataformas (Chatwoot, Typebot, OpenAI, Dify, n8n, Flowise, evoai, evolutionBot). Wuzapi exige você construir esse adapter por fora.
4. **Observability stack** — Prometheus `/metrics` nativo, Sentry SDK plugado, Grafana dashboard pronto. Wuzapi tem `/health` mas zero Prometheus.
5. **Event brokers diversos** — 7 vs 2. Importa se sua arquitetura usa Kafka/NATS/SQS.

### O que não muda entre os dois

- **Nenhum dos dois escala horizontalmente sem sharding manual.** Ambos são stateful single-node.
- **Webhook retry in-process em ambos.** Pra delivery garantida (at-least-once), precisa de fila externa em ambos os casos.
- Cobertura de envio básico (texto/mídia/grupos) — funcionalmente equivalentes.

### Pontos de atenção pra decisão (não decidem sozinhos)

- **Versão Baileys 7.0-rc.9** no evolution-api é release candidate. Em produção isso é incomum. Vale checar o que vem no próximo GA e qual é a cadência.
- **wuzapi com pseudo-version do whatsmeow** (Git SHA pinado) significa que upgrades exigem rebuild de `go.mod` e validação manual — sem release notes formais.
- **Templates Meta**: rota está comentada em `wuzapi/routes.go:111` mas o handler `SendTemplate` existe em `handlers.go:3052`. É um descomentar de linha + teste pra ter paridade nesse ponto.
- **Persistência de mensagem opcional no wuzapi**: a tabela `message_history` existe (migrations.go:176), mas a captura é controlada por flag por user (`history` field). Pode ser ativada se necessário, com tradeoff de DB size.

### Riscos da migração (em ordem de severidade)

1. **Perda de chatbot integrations embutidas** — se você usa o Chatwoot/Typebot/OpenAI native do evolution, precisa portar pra integração externa.
2. **Perda parcial de metadata de mensagens** — contatos e chat settings sobrevivem (whatsmeow persiste no Postgres por default). O que falta no wuzapi: `MessageUpdate` (rastreio de delivery status, edits, deletes), entidade `Chat` agregada (nome + last msg + unread). Se você consulta isso hoje via evolution `findChats` / `findMessages` com filtros ricos, precisa agregar manualmente ou estender o schema.
3. **Perda de canal Meta Cloud API** — se você tem instâncias rodando via API oficial, não tem equivalente direto.
4. **Quebra de payload de webhook** — formato dos eventos é diferente (`constants.go` wuzapi vs `EventEmitter2` evolution). Refactor obrigatório nos consumidores.
5. **Perda de Prometheus** — precisa instrumentar ou viver com `/health` + log aggregation.

### Pontos onde wuzapi pode reduzir o gap com esforço pequeno

- Implementar `/metrics` Prometheus (Go tem `prometheus/client_golang`, é trivial).
- Descomentar rota de Template (`routes.go:111`) e testar.
- Configurar `history` por padrão nos users novos pra persistir mensagens.
- Adicionar Sentry SDK Go (5 linhas em `main.go`).

---

## Apêndice — Evidências por path

### wuzapi
- Rotas: `routes.go` (todas declaradas em ~165 linhas)
- Handlers: `handlers.go` (7.127 LoC, ~75 funções)
- Sessão WhatsApp: `wmiau.go` (gerenciamento whatsmeow)
- Schema DB: `migrations.go` (9 migrations versionadas)
- Caches/concorrência: `clients.go`
- Webhook: `helpers.go:248-390`
- RabbitMQ: `rabbitmq.go` (298 LoC)
- S3: `s3manager.go` (436 LoC)
- Stdio JSON-RPC: `stdio.go` (629 LoC)
- Healthcheck: `handlers.go:46-119`

### evolution-api
- Rotas: `src/api/routes/index.router.ts`
- Sessão WhatsApp: `src/api/integrations/channel/whatsapp/whatsapp.baileys.service.ts`
- Schema DB: `prisma/postgresql-schema.prisma` (36 models)
- Metrics: `src/api/routes/index.router.ts:93-161`
- Sentry: `src/main.ts:1`
- Healthcheck: `Dockerfile:58` + k8s tcpSocket
- Chatbot integrations: `src/api/integrations/chatbot/{chatwoot,typebot,openai,dify,n8n,evoai,flowise,evolutionBot}/`
- Channels: `src/api/integrations/channel/{baileys,meta,evolution}/`
- Event brokers: `src/api/integrations/event/{rabbitmq,nats,sqs,kafka,pusher,websocket,webhook}/`
