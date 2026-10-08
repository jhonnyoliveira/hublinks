# Modelo de dados (Fase 1)

Convenções: IDs `uuid` (v7, gerados na aplicação); datas `timestamptz` em UTC; nomes em inglês
como no escopo. "Domínio" = toda tabela com `org_id`; toda consulta filtra por `org_id`.
`NIL` = `00000000-0000-0000-0000-000000000000`.

Estado de ciclo de vida (marketplaces, channels, affiliate_links):

```text
ativo ──excluir──▶ lixeira (deleted_at) ──30 dias (job)──▶ excluído definitivamente (purged_at)
  ▲                    │
  └─────restaurar──────┘
```

Itens em lixeira ou excluídos não aparecem nas listagens padrão; a lixeira tem visão própria.
Linhas nunca são removidas fisicamente (preservam nomes no histórico).

## organizations

| Campo | Tipo | Regras |
|---|---|---|
| id | uuid PK | |
| name | text | obrigatório |
| slug | text | único |
| created_at | timestamptz | |

Global (é a raiz do tenant). Uma org padrão criada na partida se não existir.

## users

| Campo | Tipo | Regras |
|---|---|---|
| id | uuid PK | |
| email | text | único, armazenado em minúsculas |
| password_hash | text | argon2id (formato PHC) |
| created_at, deleted_at | timestamptz | |

Global. Administrador inicial criado de `ADMIN_EMAIL`/`ADMIN_PASSWORD` somente se a tabela
estiver vazia.

## memberships

`(user_id, org_id)` PK, `role` ∈ {`admin`, `maintainer`}; fase 1 usa só `admin`.

## sessions

| Campo | Tipo | Regras |
|---|---|---|
| token_hash | bytea PK | SHA-256 do token do cookie |
| user_id | uuid FK | |
| org_id | uuid FK | org ativa da sessão |
| csrf_token | text | aleatório, por sessão |
| created_at, last_seen_at, expires_at | timestamptz | expira em 7 dias sem uso |

## marketplaces

| Campo | Tipo | Regras |
|---|---|---|
| id | uuid PK | |
| org_id | uuid FK | |
| name | text | 1–80 caracteres; único por org entre não excluídos definitivamente |
| shorten_policy | text | `shorten` \| `direct` |
| created_at, updated_at, deleted_at, purged_at | timestamptz | |

Exclusão recusada se houver `affiliate_links` com `deleted_at IS NULL` (FR-008c).

## channels

| Campo | Tipo | Regras |
|---|---|---|
| id | uuid PK | |
| org_id | uuid FK | |
| name | text | 1–60 caracteres |
| segment | text | `^[a-z0-9][a-z0-9-]{0,19}$`, minúsculo; não reservado (`api`, `admin`, `p`, `static`, `healthz`, `beacon`, `privacidade`) |
| created_at, updated_at, deleted_at, purged_at | timestamptz | |

- Índice único `(org_id, segment) WHERE purged_at IS NULL`: o segmento fica reservado enquanto
  o canal estiver na lixeira (FR-008d).
- Segmento e código não colidem: as rotas têm quantidades diferentes de partes
  (`/{code}` e `/{segment}/{code}`).
- Exclusão de canal (FR-008c): permitida; a confirmação informa quantos links ativos terão suas
  URLs desse canal invalidadas. A trava por links ativos vale só para marketplaces.

## affiliate_links

| Campo | Tipo | Regras |
|---|---|---|
| id | uuid PK | |
| org_id | uuid FK | |
| marketplace_id | uuid FK | marketplace não excluído na criação/edição |
| product_id | uuid NULL | sem uso na fase 1 (sem FK até a fase 2) |
| title | text | 1–200 caracteres |
| image_url | text NULL | `http`/`https` |
| destination_url | text | obrigatório, `http`/`https`, até 2048 caracteres, com host |
| shorten_policy_override | text NULL | `shorten` \| `direct` |
| active | boolean | padrão `true` |
| created_at, updated_at, deleted_at, purged_at | timestamptz | |

Política efetiva = `shorten_policy_override` ou `marketplaces.shorten_policy`. Restaurar um link
exige marketplace não excluído.

## short_codes

| Campo | Tipo | Regras |
|---|---|---|
| code | char(7) PK | `^[0-9a-z]{7}$`, ≠ palavras reservadas |
| org_id | uuid FK | |
| target_type | text | `affiliate_link` (fase 2 acrescenta `product`) |
| target_id | uuid | |
| created_at | timestamptz | |

Global na resolução (PK sem `org_id`), mas carrega `org_id` para as operações seguintes.
Nunca apagado (FR-008b). Um por link.

## click_events

| Campo | Tipo | Regras |
|---|---|---|
| id | uuid PK | v7 |
| occurred_at | timestamptz | momento do acesso (não da gravação) |
| org_id | uuid | |
| event_type | text | `click` (fase 2: `view`) |
| code | char(7) | |
| target_type, target_id | text, uuid | |
| channel_id | uuid NULL | NULL = URL curta sem canal |
| visitor_id | bytea(32) | HMAC; nunca IP |
| is_unique_url, is_unique_target, is_bot | boolean | |
| referer | text NULL | truncado em 1024 |
| user_agent | text | truncado em 512 |

Índices: `(org_id, occurred_at)`, `(target_id, occurred_at)`. Sem FKs (gravação em lote rápida;
integridade garantida pela origem). Retenção: reais 395 dias, robôs 30 dias, somente dias em
`aggregated_days`.

## visitor_seen

| Campo | Tipo | Regras |
|---|---|---|
| target_type, target_id | text, uuid | |
| scope | text | `url` \| `target` (decidido; research R5) |
| channel_id | uuid | `scope=url`: canal ou `NIL`; `scope=target`: sempre `NIL` |
| visitor_id | bytea(32) | |
| org_id | uuid | acrescentado (constituição IV) |
| first_seen_at | timestamptz | |

PK `(target_type, target_id, scope, channel_id, visitor_id)`. Apagado somente na exclusão
definitiva do alvo. Não recebe linhas de eventos de robô.

## click_daily

| Campo | Tipo | Regras |
|---|---|---|
| org_id | uuid | |
| day | date | dia em `REPORT_TZ` |
| event_type | text | |
| target_type, target_id | text, uuid | |
| channel_id | uuid | `NIL` para "sem canal" |
| clicks, unique_url, unique_target | integer | somas sem robôs |

PK `(org_id, day, event_type, target_type, target_id, channel_id)`. Mantido indefinidamente.
Alcance único do alvo = `SUM(unique_target)` em todos os canais.

## aggregated_days

`(org_id, day)` PK, `aggregated_at timestamptz`. Marca dias consolidados; condição da limpeza.

## app_settings

`key text PK`, `value text`. Guarda `report_timezone` fixado na primeira agregação.

## Regras de validação (mapa para a spec)

| Regra | Requisito |
|---|---|
| URL de destino/imagem só `http`/`https` com host | FR-006 |
| Código 7 `[0-9a-z]`, único, não reservado, não reutilizado | FR-007, FR-008b |
| Segmento único por org, não reservado, reservado na lixeira | FR-004, FR-008d |
| Marketplace com links ativos não excluível | FR-008c |
| Toda consulta de domínio com `org_id` | FR-009 |
| Robôs fora de `visitor_seen` e de `click_daily` | FR-021, FR-027 |
