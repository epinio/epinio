# Grafana Cloud setup for Epinio fleet telemetry

This runbook covers everything you need to configure **on the Grafana Cloud side** before Epinio can push daily fleet metrics (Epinio version, Kubernetes version, app/namespace/service counts, and related inventory).

The Epinio cluster will push metrics via **OpenTelemetry Protocol (OTLP) over HTTP** to Grafana Cloud. Grafana Cloud stores those metrics in **Mimir** (Prometheus-compatible). You query and visualize them in **Grafana**.

---

## Table of contents

1. [What you are setting up](#1-what-you-are-setting-up)
2. [Prerequisites](#2-prerequisites)
3. [Step 1 — Create or select a Grafana Cloud stack](#3-step-1--create-or-select-a-grafana-cloud-stack)
4. [Step 2 — Create an access policy and token](#4-step-2--create-an-access-policy-and-token)
5. [Step 3 — Collect connection details for Epinio](#5-step-3--collect-connection-details-for-epinio)
6. [Step 4 — Verify the OTLP endpoint accepts metrics (manual test)](#6-step-4--verify-the-otlp-endpoint-accepts-metrics-manual-test)
7. [Step 5 — Confirm metrics in Grafana Explore](#7-step-5--confirm-metrics-in-grafana-explore)
8. [Step 6 — Create stakeholder dashboards](#8-step-6--create-stakeholder-dashboards)
9. [Step 7 — Optional: alert rules](#9-step-7--optional-alert-rules)
10. [Step 8 — Operational hygiene](#10-step-8--operational-hygiene)
11. [Information to hand off to the Epinio / Helm work](#11-information-to-hand-off-to-the-epinio--helm-work)
12. [Troubleshooting](#12-troubleshooting)
13. [References](#13-references)

---

## 1. What you are setting up

```text
┌──────────────────────────────┐         OTLP/HTTP (daily)          ┌─────────────────────┐
│  Epinio cluster              │  ───────────────────────────────►  │  Grafana Cloud      │
│                              │                                    │                     │
│  CronJob (8 AM)              │                                    │  OTLP gateway       │
│       │                      │                                    │       │             │
│       ▼                      │                                    │       ▼             │
│  POST /telemetry/publish     │                                    │  Mimir (metrics)    │
│  (epinio-server)             │                                    │       │             │
│       │                      │                                    │       ▼             │
│       └── OTLP exporter ─────┼────────────────────────────────────┤  Grafana UI         │
└──────────────────────────────┘                                    └─────────────────────┘
```

**Grafana Cloud responsibilities:**

| Component | Role |
|-----------|------|
| OTLP gateway | Receives pushed metrics from Epinio |
| Mimir | Stores time-series metrics |
| Grafana | Dashboards and Explore for stakeholders |
| Access policy + token | Authenticates Epinio pushes |

You do **not** need to install Alloy or Kubernetes Monitoring on the cluster for this fleet-telemetry path (those are separate if you later want full infra observability).

---

## 2. Prerequisites

- [ ] Grafana Cloud account — [https://grafana.com/auth/sign-up/create-user](https://grafana.com/auth/sign-up/create-user) (free tier is fine for development)
- [ ] Permission to create **access policies** and **tokens** in the stack
- [ ] Agreement on **data classification** — fleet metrics are aggregate counts and version strings; do not push customer PII or raw logs through this path
- [ ] A private place to store the token (password manager or team secrets vault — **never git**)

---

## 3. Step 1 — Create or select a Grafana Cloud stack

1. Sign in at [https://grafana.com](https://grafana.com).
2. Open **My Account** (profile menu, top right).
3. Under **Stacks**, either:
   - **Create** a new stack, e.g. `epinio-fleet-telemetry-dev`, or
   - Select an existing dev/test stack.
4. Click **Launch** to open that stack's Grafana UI.

**Note the stack name and region** (e.g. `prod-eu-west-0`) — OTLP gateway URLs are region-specific.

---

## 4. Step 2 — Create an access policy and token

Epinio needs a token that can **write metrics** via OTLP.

### 4.1 Open Access Policies

1. In Grafana Cloud portal (My Account, not the Grafana UI): go to **Security** → **Access Policies**.
2. Select your stack if prompted.

### 4.2 Create a policy

1. Click **Create access policy**.
2. Suggested name: `epinio-fleet-telemetry-push`.
3. Scopes — enable at minimum:
   - **`metrics:write`** — required for OTLP metric ingestion into Mimir

   Optional (only if you expand scope later):

   - `logs:write` — if Epinio also pushes logs to Loki
   - `traces:write` — if you add distributed tracing later

4. Save the policy.

### 4.3 Create a token

1. On the policy, click **Add token**.
2. Name: `epinio-fleet-telemetry-<cluster-or-env>` (e.g. `epinio-fleet-telemetry-minikube-dev`).
3. Set expiration:
   - **Dev/spike:** 30–90 days
   - **Production:** follow your org rotation policy; prefer short-lived tokens with documented rotation
4. Click **Create token**.
5. **Copy the token immediately** — it starts with `glc_` and is shown only once.

Store it securely. You will map it into a Kubernetes Secret on the cluster side (not in this doc).

---

## 5. Step 3 — Collect connection details for Epinio

You need three values for the Epinio OTLP exporter and Helm chart:

| Setting | Where to find it | Example shape |
|---------|------------------|---------------|
| **OTLP endpoint (HTTP)** | Stack → **Connections** → **OpenTelemetry** → **OTLP endpoint** | `https://otlp-gateway-prod-eu-west-0.grafana.net/otlp` |
| **Instance ID** (username) | Same page, or Prometheus/Loki connection details — numeric stack instance ID | `123456` |
| **Token** (password) | Token created in Step 2 | `glc_eyJ...` |

### 5.1 Find the OTLP gateway URL

1. Grafana Cloud portal → your stack → **Connections** (or **Send data**).
2. Choose **OpenTelemetry** / **OTLP**.
3. Copy the **HTTP** endpoint. The metrics path is typically:

   ```text
   https://otlp-gateway-prod-<region>.grafana.net/otlp/v1/metrics
   ```

   Some SDKs accept the base URL (`.../otlp`) and append `/v1/metrics` automatically — confirm against the snippet Grafana shows for **Go** or **HTTP**.

### 5.2 Authentication

Grafana Cloud OTLP uses **HTTP Basic Auth**:

- **Username:** stack instance ID (numeric)
- **Password:** the `glc_` access policy token

Alternatively, some clients use header:

```text
Authorization: Bearer <glc_token>
```

Use whichever format your OTLP client documents; Grafana Cloud accepts the basic-auth style shown in their connection wizard.

### 5.3 Private worksheet (fill in and keep offline)

```text
Stack name:           _______________________________
Region:               _______________________________
OTLP HTTP endpoint:   _______________________________
Instance ID:          _______________________________
Token (glc_...):      _______________________________  ← do not commit
Environment label:    _______________________________  e.g. dev / staging / prod
Cluster name label:   _______________________________  e.g. epinio-dev-01
```

The **cluster name** and **environment** become metric labels so you can filter multiple Epinio installations in one Grafana stack.

---

## 6. Step 4 — Verify the OTLP endpoint accepts metrics (manual test)

Before wiring Epinio, prove the Cloud path works with a one-off push.

### Option A — Use Grafana's connection test (if available)

In **Connections → OpenTelemetry**, some stacks offer **Test connection**. Run it after pasting your token.

### Option B — curl with a minimal OTLP payload

Grafana documents exact examples in the connection UI. A typical check:

1. Install [otel-cli](https://github.com/equinix-labs/otel-cli) or use a small Go/Python OTLP test program from Grafana's docs.
2. Push a test gauge, e.g. `epinio_test_connection = 1`, with resource attribute `service.name=epinio-telemetry-test`.

If you get **HTTP 200** (or **202**), ingestion works. **401/403** means wrong token or scopes. **404** usually means wrong URL path.

### Option C — Wait for Epinio endpoint, then trigger once manually

Skip this until the Epinio `POST /telemetry/publish` endpoint exists; use Steps 6–7 below to validate end-to-end.

---

## 7. Step 5 — Confirm metrics in Grafana Explore

After the first successful push (manual test or Epinio cron):

1. Open your stack's **Grafana** UI.
2. Go to **Explore** (compass icon).
3. Select the **Prometheus** data source (Grafana Cloud automatically wires this to Mimir).
4. Set time range to **Last 15 minutes** (or since your test push).
5. Run queries:

```promql
# Any Epinio fleet metric (names are examples — align with implementation)
{__name__=~"epinio_.*"}

# Test metric from manual push
epinio_test_connection

# Once live — version info
epinio_build_info

# Once live — inventory gauges
epinio_inventory_applications
epinio_inventory_namespaces
epinio_inventory_services
```

6. If empty:
   - Wait 1–2 minutes (ingestion delay).
   - Check **Metrics → Usage** or **Cardinality** in Cloud portal for incoming series.
   - Re-check token, endpoint URL, and that the push returned success.

---

## 8. Step 6 — Create stakeholder dashboards

Build dashboards in Grafana → **Dashboards** → **New** → **New dashboard**.

### 8.1 Recommended dashboard: Epinio Fleet Overview

**Audience:** product, leadership, support  
**Refresh:** daily (matches cron) or 1h  
**Variables to add:**

| Variable | Type | Query / values |
|----------|------|----------------|
| `cluster` | Query | `label_values(epinio_build_info, cluster)` |
| `environment` | Query | `label_values(epinio_build_info, environment)` |

**Suggested panels:**

| Panel title | Type | Example PromQL |
|-------------|------|----------------|
| Active Epinio installations | Stat | `count(count by (cluster) (epinio_build_info))` |
| Epinio version distribution | Pie chart | `count by (epinio_version) (epinio_build_info)` |
| Kubernetes version distribution | Bar chart | `count by (kubernetes_version) (epinio_cluster_info)` |
| Total applications (selected cluster) | Stat | `max(epinio_inventory_applications{cluster="$cluster"})` |
| Total namespaces | Stat | `max(epinio_inventory_namespaces{cluster="$cluster"})` |
| Total services | Stat | `max(epinio_inventory_services{cluster="$cluster"})` |
| Applications over time | Time series | `max(epinio_inventory_applications{cluster="$cluster"})` |
| Last successful report | Stat | `max(epinio_telemetry_last_success_timestamp{cluster="$cluster"})` |

Metric names above are **proposed** — adjust to match the Epinio implementation.

### 8.2 Dashboard: Epinio Adoption trends

For stakeholders who care about growth over weeks/months:

- Time series of `epinio_inventory_applications`, `epinio_inventory_namespaces`, `epinio_inventory_services`
- Group by `cluster` or `environment`
- Use **Last** aggregation (gauges pushed once per day)

### 8.3 Export and share

1. **Share → Export** → save JSON to your team's docs or (later) a Helm chart ConfigMap.
2. Set folder permissions: **Viewer** for stakeholders, **Editor** for platform team.
3. Optional: **Public dashboard** only if data is non-sensitive and policy allows — usually keep internal.

---

## 9. Step 7 — Optional: alert rules

Useful alerts once daily pushes run:

| Alert | Condition | Purpose |
|-------|-----------|---------|
| Telemetry push stopped | No `epinio_telemetry_last_success_timestamp` increase in 26h | Cron or endpoint failure |
| Unknown Epinio version spike | New `epinio_version` label appears | Fleet drift / upgrade tracking |

Create in Grafana → **Alerting** → **Alert rules** → select Prometheus data source.

Example (adjust metric name):

```promql
time() - max(epinio_telemetry_last_success_timestamp) > 93600
```

(93600 seconds = 26 hours — allows for timezone/cron skew.)

---

## 10. Step 8 — Operational hygiene

### Token lifecycle

- [ ] Record token creation date and expiry in team runbook
- [ ] Plan rotation before expiry: create new token → update K8s Secret → verify push → revoke old token
- [ ] Never paste tokens in Jira, Slack, or PR comments

### Cost and cardinality

Fleet telemetry should use **low-cardinality labels** only:

| Good labels | Avoid as labels |
|-------------|-----------------|
| `cluster`, `environment`, `epinio_version`, `kubernetes_version`, `platform` | app names, user names, URLs, request IDs |

High-cardinality labels increase Mimir cost. Keep per-app detail in logs or separate tooling if needed.

### Retention

Check stack **Metrics retention** (Cloud portal → stack settings). Default is often 13–14 months on paid tiers; shorter on free tier. Align with how long stakeholders need historical adoption trends.

### Cleanup when decommissioning a cluster

1. Revoke that cluster's token (or remove from shared policy).
2. Remove dashboard variables referencing the cluster (series age out per retention).

---

## 11. Information to hand off to the Epinio / Helm work

When Grafana Cloud is ready, provide the platform team:

```yaml
# Example shape for Kubernetes Secret / Helm values (cluster-side — not in git)
telemetry:
  enabled: true
  otlp:
    endpoint: "https://otlp-gateway-prod-<region>.grafana.net/otlp"
    # SDK may append /v1/metrics
  auth:
    username: "<INSTANCE_ID>"
    password: "<glc_TOKEN>"   # from secret
  labels:
    cluster: "epinio-dev-01"
    environment: "dev"
```

They will wire:

- Epinio server OTLP exporter
- CronJob at `0 8 * * *` calling the publish endpoint
- Kubernetes Secret holding the token

---

## 12. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| HTTP 401 / 403 on OTLP push | Wrong token, expired token, or missing `metrics:write` | Recreate token; verify policy scopes |
| HTTP 404 | Wrong OTLP URL or missing `/v1/metrics` path | Copy URL from Connections page exactly |
| Push succeeds, no data in Explore | Wrong Prometheus data source; delay; querying wrong label | Use Cloud's default Prometheus DS; wait 2 min; try `{__name__=~".+"}` |
| Duplicate series | Same cluster label from two installs | Use unique `cluster` label per installation |
| Costs rising fast | Too many labels or high scrape cardinality elsewhere | Audit label set; fleet metrics should be < 20 active series per cluster |
| Dashboard empty after 1 day | Cron not running; endpoint failing; timezone mismatch | Check CronJob history; Epinio server logs; confirm UTC vs local schedule |

---

## 13. References

- [Grafana Cloud — Send OTLP data](https://grafana.com/docs/grafana-cloud/send-data/otlp/)
- [Grafana Cloud — Access policies](https://grafana.com/docs/grafana-cloud/account-management/authentication-and-permissions/access-policies/)
- [OpenTelemetry — OTLP specification](https://opentelemetry.io/docs/specs/otlp/)
- [Prometheus query basics (for dashboards)](https://prometheus.io/docs/prometheus/latest/querying/basics/)

---

## Checklist (Grafana Cloud side complete when all ticked)

- [ ] Stack created or selected
- [ ] Access policy `epinio-fleet-telemetry-push` with `metrics:write`
- [ ] Token created and stored securely
- [ ] OTLP HTTP endpoint URL recorded
- [ ] Instance ID recorded
- [ ] Manual OTLP test push succeeded (optional but recommended)
- [ ] Explore shows test or first real Epinio metrics
- [ ] Fleet Overview dashboard created (even if stub panels)
- [ ] Stakeholders granted Viewer access
- [ ] Token expiry / rotation noted in runbook
