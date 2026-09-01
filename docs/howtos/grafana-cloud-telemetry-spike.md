# Grafana Cloud setup for Epinio telemetry (EPINIO-685)

> **Spike:** [EPINIO-685](https://jira.suse.com/) — *Define what is necessary to be setup in Grafana Cloud*  
> **Parent:** EPINIO-681  
> **Goal:** Figure out what dashboards are useful with telemetry data; use this spike for setup, finding limitations, and documenting decisions.

This guide is a step-by-step runbook for the spike. It covers Grafana Cloud account setup, shipping Kubernetes/Epinio telemetry, verifying data, proposing dashboards, and recording limitations.

---

## Table of contents

1. [Scope and outcomes](#1-scope-and-outcomes)
2. [Current Epinio observability landscape](#2-current-epinio-observability-landscape)
3. [Architecture for the spike](#3-architecture-for-the-spike)
4. [Prerequisites](#4-prerequisites)
5. [Step 1 — Create / prepare Grafana Cloud](#5-step-1--create--prepare-grafana-cloud)
6. [Step 2 — Access policy and token](#6-step-2--access-policy-and-token)
7. [Step 3 — Note stack connection details](#7-step-3--note-stack-connection-details)
8. [Step 4 — Prepare the Kubernetes cluster](#8-step-4--prepare-the-kubernetes-cluster)
9. [Step 5 — Install Kubernetes Monitoring (Grafana Alloy)](#9-step-5--install-kubernetes-monitoring-grafana-alloy)
10. [Step 6 — Verify telemetry is flowing](#10-step-6--verify-telemetry-is-flowing)
11. [Step 7 — Explore built-in dashboards](#11-step-7--explore-built-in-dashboards)
12. [Step 8 — Define Epinio-specific dashboards](#12-step-8--define-epinio-specific-dashboards)
13. [Step 9 — Optional: application observability (OTLP / Beyla)](#13-step-9--optional-application-observability-otlp--beyla)
14. [Step 10 — Relate product telemetry (upgrade-responder)](#14-step-10--relate-product-telemetry-upgrade-responder)
15. [Step 11 — Document limitations](#15-step-11--document-limitations)
16. [Step 12 — Spike exit criteria and findings template](#16-step-12--spike-exit-criteria-and-findings-template)
17. [Cleanup](#17-cleanup)
18. [References](#18-references)

---

## 1. Scope and outcomes

### In scope

- Standing up a Grafana Cloud stack suitable for Epinio spike work
- Shipping **cluster metrics**, **pod logs**, and **cluster events** from a cluster that runs Epinio
- Identifying which **dashboards** are useful given that data
- Recording **setup friction**, **cost / cardinality**, and **product gaps** (for example missing OpenTelemetry in Epinio server)

### Out of scope (unless time allows)

- Production multi-cluster fleet management
- Replacing the existing public upgrade-responder Grafana at [version.rancher.io](https://version.rancher.io)
- Full first-party OpenTelemetry instrumentation of the Epinio server (that is follow-up work; OTel is listed only as a future interest in `docs/explanations/futureplans.md`)

### Expected spike deliverables

1. Working Grafana Cloud → Alloy → Epinio cluster path
2. List of recommended dashboards (built-in + custom)
3. Written limitations and open questions
4. Clear recommendation: **infra monitoring only**, **product telemetry**, or **both**

---

## 2. Current Epinio observability landscape

Know what already exists so the spike does not reinvent it.

| Capability | Today | Where |
|------------|--------|--------|
| App CPU / memory in CLI / UI | Kubernetes Metrics API (`metrics.k8s.io`) | Epinio server / workload code |
| App and server logs | `epinio app logs`, server zap logging | Cluster + CLI |
| Product / adoption metrics | Upgrade Responder → InfluxDB → Grafana | Helm chart `helm-charts/chart/upgrade-responder`, docs `docs/howtos/upgrade-responder.md` |
| Public dashboard | Version / platform / geography of active nodes | https://version.rancher.io (Epinio Upgrade Checker) |
| OpenTelemetry | Not implemented as a product feature | Future interest only |

**Implication for Grafana Cloud:** without new instrumentation, Cloud will primarily show **Kubernetes infrastructure and pod logs**. Rich RED metrics (rate / errors / duration) for the Epinio API and staging pipeline need either Prometheus scrapes, Beyla/eBPF, or OTel later.

---

## 3. Architecture for the spike

```text
┌─────────────────────────────┐
│  Kubernetes cluster         │
│  ┌─────────┐  ┌──────────┐  │     OTLP / remote_write / Loki push
│  │ Epinio  │  │  Alloy   │──┼──────────────────────────────────┐
│  │ server  │  │ (k8s-    │  │                                  │
│  │ + apps  │  │ monitor) │  │                                  ▼
│  └─────────┘  └──────────┘  │                     ┌────────────────────┐
│       │ metrics.k8s.io      │                     │  Grafana Cloud     │
│       │ pod logs / events   │                     │  Mimir | Loki |    │
└───────┴─────────────────────┘                     │  Tempo | Grafana   │
                                                    └────────────────────┘
```

**Recommended path for the spike:** Grafana Cloud **Kubernetes Monitoring** Helm chart (`grafana/k8s-monitoring`), which deploys **Grafana Alloy** collectors. Prefer this over a hand-rolled OpenTelemetry Collector unless the team already standardizes on OTel Collector.

Official docs: [Configuration steps for Kubernetes Monitoring with Helm](https://grafana.com/docs/grafana-cloud/monitor-infrastructure/kubernetes-monitoring/configuration/helm-chart-config/hc-config-steps/).

---

## 4. Prerequisites

Before starting:

- [ ] Grafana Cloud account (free tier is enough for a spike; watch host/container hour billing once K8s monitoring is enabled)
- [ ] Role that can install alert/recording rules in the Grafana Cloud stack (Admin for the Kubernetes Monitoring activation flow)
- [ ] A Kubernetes cluster with Epinio installed (local k3d/kind, or a shared dev cluster)
- [ ] `kubectl` configured to that cluster
- [ ] Helm 3.x installed
- [ ] Ability to create namespaces and cluster-scoped RBAC (kube-state-metrics, Alloy, etc.)
- [ ] Agreement on data classification: this ticket is marked **Confidential** — do not ship production customer logs or secrets into a personal Cloud stack

Suggested cluster label for the spike:

```text
cluster.name = epinio-spike-<your-name-or-date>
```

---

## 5. Step 1 — Create / prepare Grafana Cloud

### 5.1 Sign in and pick a stack

1. Open [https://grafana.com](https://grafana.com) and sign in.
2. Open **My Account** → select or create a **stack** (for example `epinio-telemetry-spike`).
3. Open the stack’s Grafana UI (**Launch**).

### 5.2 Activate Kubernetes Monitoring

1. In Grafana Cloud, go to **Observability** → **Kubernetes** (or **Infrastructure** → **Kubernetes Monitoring**, depending on UI version).
2. Start **Cluster configuration** / **Add cluster**.
3. Complete **Backend and distribution**:
   - Click **Install** for the required **alert rules and recording rules**.
   - Choose your platform (usually **Kubernetes** for k3d/kind/GKE/EKS-on-EC2).
4. Continue to the access token step (next section).

> **Why recording rules matter:** Kubernetes Monitoring workload views depend on them. If dashboards look empty later, re-check that backend rules installed successfully.

---

## 6. Step 2 — Access policy and token

Alloy needs credentials to write telemetry.

### Option A — Create token in the wizard (simplest for spike)

1. Choose **Create a new token**.
2. Name it, for example `epinio-spike-alloy`.
3. Set an expiration (recommended for spikes: 30–90 days).
4. Create and **store the token somewhere safe** (password manager). You will not see it again.

Default scopes from the wizard typically include write scopes Alloy needs (for example `set:alloy-data-write`). Prefer the wizard defaults for the spike.

### Option B — Kubernetes Secret (better hygiene)

1. In the wizard, choose **Use a stored Kubernetes Secret**.
2. Create the secret in the install namespace (example `monitoring`):

```bash
kubectl create namespace monitoring

kubectl create secret generic grafana-cloud-credentials \
  --namespace monitoring \
  --from-literal=username='<GRAFANA_CLOUD_INSTANCE_ID>' \
  --from-literal=password='<ACCESS_POLICY_TOKEN>'
```

3. Point the wizard / Helm values at that secret name and namespace.

> **Never** commit the token to git, Helm values checked into the repo, or screenshots in public issues.

---

## 7. Step 3 — Note stack connection details

From Grafana Cloud → your stack → **Connections** / **Prometheus** / **Loki** (or the Kubernetes Monitoring wizard summary), record:

| Item | Example placeholder | Used for |
|------|---------------------|----------|
| Stack / instance name | `epinio-telemetry-spike` | UI navigation |
| Prometheus remote write URL | `https://prometheus-prod-XX.grafana.net/api/prom/push` | Metrics |
| Prometheus / metrics user (instance ID) | numeric ID | Basic auth username |
| Loki push URL | `https://logs-prod-XX.grafana.net/loki/api/v1/push` | Logs |
| Loki user | numeric ID | Basic auth username |
| Tempo OTLP endpoint (if traces) | `https://tempo-prod-XX.grafana.net:443` | Traces |
| Access policy token | `glc_...` | Password for all of the above |

If you use the **wizard-generated Helm command**, these values are often embedded for you — still keep a private note of the instance IDs and URLs for troubleshooting.

---

## 8. Step 4 — Prepare the Kubernetes cluster

### 8.1 Confirm Epinio is running

```bash
kubectl get pods -n epinio
kubectl get svc -n epinio
epinio info   # if CLI is configured
```

You should see `epinio-server` (and related components) healthy.

### 8.2 Deploy a sample app (optional but useful for dashboards)

```bash
epinio target workspace   # or your namespace
epinio app push -n sample --path <path-to-sample-app>
```

Having at least one app makes pod CPU/memory and log panels more meaningful.

### 8.3 Decide signal set for the spike

| Signal | Spike recommendation | Notes |
|--------|----------------------|-------|
| Cluster metrics | **On** | Baseline; powers K8s overview |
| Cluster events | **On** | Useful for staging / schedule failures |
| Pod logs | **On** (Epinio + app namespaces) | Watch volume / PII |
| Cost metrics (OpenCost) | Optional | Extra pods; good if FinOps is a question |
| Energy metrics (Kepler) | Off for spike | Usually not needed |
| OTel receivers | On if testing app OTLP | See Step 9 |
| Beyla zero-code | Optional | Fast RED signals; may increase billing |

For a first pass, use the wizard’s **Lightweight** or **Standard** setup tier:

- **Lightweight** — smaller clusters, faster install, fewer signals
- **Standard** — closer to production shape

---

## 9. Step 5 — Install Kubernetes Monitoring (Grafana Alloy)

### 9.1 Preferred: copy command from Grafana Cloud wizard

1. In the wizard **Deployment** step:
   - **Cluster name:** `epinio-spike-<id>` (lowercase, digits, hyphens)
   - **Namespace:** `monitoring` (or wizard default)
2. Add the Helm repo if needed:

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
```

3. Copy the generated `helm upgrade --install ...` command from the UI and run it with your kubecontext set to the spike cluster.
4. Click **Test connection** in the wizard when prompted.

### 9.2 Manual install sketch (if not using wizard)

Adapt URLs, usernames, and the token from Step 3. Exact value keys evolve with chart versions — prefer the wizard output when possible.

```bash
helm upgrade --install grafana-k8s-monitoring grafana/k8s-monitoring \
  --namespace monitoring --create-namespace \
  --set cluster.name=epinio-spike-01 \
  --set destinations.grafana-cloud-metrics.url='https://prometheus-prod-XX.grafana.net/api/prom/push' \
  --set destinations.grafana-cloud-metrics.auth.username='<METRICS_INSTANCE_ID>' \
  --set destinations.grafana-cloud-metrics.auth.password='<TOKEN>' \
  --set destinations.grafana-cloud-logs.url='https://logs-prod-XX.grafana.net/loki/api/v1/push' \
  --set destinations.grafana-cloud-logs.auth.username='<LOKI_INSTANCE_ID>' \
  --set destinations.grafana-cloud-logs.auth.password='<TOKEN>' \
  --set clusterMetrics.enabled=true \
  --set clusterEvents.enabled=true \
  --set podLogsViaLoki.enabled=true
```

> Chart value structure changes across releases. If this sketch fails, fall back to the **wizard-generated command** or export values from Grafana Cloud (**Retrieve Helm values** in the docs).

### 9.3 Confirm collectors are up

```bash
kubectl get pods -n monitoring
kubectl get daemonset,deploy,statefulset -n monitoring
kubectl logs -n monitoring -l app.kubernetes.io/name=k8s-monitoring --tail=100
```

Expect Alloy (and possibly kube-state-metrics / node-exporter) pods `Running`. Investigate `CrashLoopBackOff` or auth errors in logs immediately.

---

## 10. Step 6 — Verify telemetry is flowing

### 10.1 Cluster status in Grafana Cloud

1. Open **Kubernetes Monitoring** → your cluster.
2. Open **Metrics status** / **Cluster status**.
3. Confirm sources show healthy (cluster metrics, events, pod logs as enabled).

### 10.2 Metrics Explore (PromQL)

In Grafana → **Explore** → Prometheus datasource:

```promql
# Nodes reporting
count(kube_node_info{cluster="epinio-spike-01"})

# Pods in epinio namespace
count(kube_pod_info{namespace="epinio", cluster="epinio-spike-01"})

# Container CPU for epinio-server
rate(container_cpu_usage_seconds_total{namespace="epinio", pod=~"epinio-server.*"}[5m])
```

Replace `cluster=` with your cluster name label if the chart uses a different label key (often `cluster` or `cluster_name`).

### 10.3 Logs Explore (LogQL)

```logql
{namespace="epinio"} |= ``
{namespace="epinio", pod=~"epinio-server.*"} |= "error" or "Error" or "ERROR"
```

### 10.4 Events

In Kubernetes Monitoring UI, open **Events** for the cluster, or query Loki if events are shipped as log streams (depends on chart config). Confirm you can see normal noise (scheduling, probes) and then induce a failure (bad image) to see it appear.

**Spike note:** Record how long data took to appear (often a few minutes).

---

## 11. Step 7 — Explore built-in dashboards

Before building custom dashboards, inventory what Cloud already gives you.

1. **Kubernetes Monitoring** navigation:
   - Cluster overview (nodes, capacity, health)
   - Namespaces → `epinio` and app namespaces
   - Workloads → `epinio-server`, staging jobs, app deployments
2. **Drilldown / Explore** apps (Metrics, Logs, Traces) if available on your plan.
3. Save links to any built-in views that already answer spike questions.

**Spike question to answer:** *Which built-in views are enough for ops, and which gaps need Epinio-specific dashboards?*

---

## 12. Step 8 — Define Epinio-specific dashboards

Create dashboards only where built-ins are weak. Use Grafana → **Dashboards** → **New** → add panels with variables:

Suggested dashboard variables:

- `cluster`
- `namespace` (Epinio workspaces / app namespaces)
- `app` (from pod or deployment labels; Epinio apps typically carry Epinio-specific labels — confirm with `kubectl get deploy -n <ns> --show-labels`)

### 12.1 Recommended dashboard set

#### A. Epinio control plane health

**Audience:** maintainers / SRE  

| Panel | Signal | Why useful |
|-------|--------|------------|
| `epinio-server` replicas ready | `kube_deployment_status_replicas_*` | Availability |
| Server CPU / memory | `container_*` | Capacity / leaks |
| Restart count | `kube_pod_container_status_restarts_total` | Crash loops |
| Server logs (error rate) | Loki count over time | Correlate with restarts |
| API ingress / service health | Probe or HTTP metrics if present | External reachability |

#### B. Staging and deploy pipeline

**Audience:** platform engineers  

| Panel | Signal | Why useful |
|-------|--------|------------|
| Staging job success / fail | Job / pod phase metrics + events | Core Epinio UX |
| Staging duration | Job start/completion timestamps or custom metrics | Performance regressions |
| Image pull / build failures | Events + logs | Common support issues |
| Concurrent staging pods | Pod count with staging labels | Queue / saturation |

> Many of these are **event/log derived** today until custom metrics exist. Document that gap.

#### C. Application workloads (developer view)

**Audience:** app developers using Epinio  

| Panel | Signal | Why useful |
|-------|--------|------------|
| Apps by namespace | Deployments / pods with app labels | Inventory |
| CPU / memory per app | Container metrics | Matches CLI metrics story |
| OOMKills / throttling | Events + `container_memory_*` | Reliability |
| Request RED (if Beyla/OTel) | Traces / HTTP metrics | True app performance |
| Route / ingress errors | Ingress controller metrics if scraped | User-facing failures |

#### D. Product / fleet telemetry (upgrade-responder parity)

**Audience:** product  

Panels analogous to the existing **Epinio Upgrade Checker** dashboard:

- Active nodes / instances over time
- Breakdown by Epinio server version
- Breakdown by Kubernetes version
- Breakdown by platform
- Geo map (if country data is still collected)

**Data source today:** InfluxDB via upgrade-responder, **not** Grafana Cloud Prometheus. See [Step 10](#14-step-10--relate-product-telemetry-upgrade-responder).

### 12.2 How to create a starter dashboard (manual)

1. Grafana → **Dashboards** → **New dashboard**.
2. Add row **Control plane**.
3. Add a **Time series** panel → Prometheus → paste:

```promql
sum(rate(container_cpu_usage_seconds_total{namespace="epinio", container="epinio-server"}[5m])) by (pod)
```

4. Add a **Logs** panel → Loki → `{namespace="epinio", pod=~"epinio-server.*"}`.
5. Add a **Stat** panel for restarts:

```promql
sum(increase(kube_pod_container_status_restarts_total{namespace="epinio", pod=~"epinio-server.*"}[1h]))
```

6. Save as `Epinio Spike — Control Plane`.
7. Export JSON (**Share** → **Export**) and attach to the Jira spike notes (without secrets).

### 12.3 Label conventions to standardize early

Agree during the spike (even if not implemented yet):

| Label | Purpose |
|-------|---------|
| `cluster` | Multi-cluster filter |
| `namespace` | Workspace isolation |
| `app.kubernetes.io/name` | Workload identity |
| Epinio app name label (confirm actual key) | Per-app dashboards |
| `epinio.io/...` (if present) | Product-specific filters |

High-cardinality labels (raw URLs, user IDs, request IDs as metric labels) will blow cost — keep them in logs/traces, not metric labels.

---

## 13. Step 9 — Optional: application observability (OTLP / Beyla)

Use this only if the spike should answer *“what do we need for Application Observability?”*

### 13.1 Enable receivers in Kubernetes Monitoring

In the Cloud wizard (or Helm values), enable:

- **OpenTelemetry receivers** on Alloy
- Optionally **Zero-code instrumentation (Beyla)** for HTTP/gRPC without code changes
- Optionally **Forward traces to application receivers**

Note OTLP endpoints shown in the UI (often an in-cluster Alloy service). Example shape:

```text
http://grafana-k8s-monitoring-alloy-receiver.monitoring.svc:4317   # gRPC
http://grafana-k8s-monitoring-alloy-receiver.monitoring.svc:4318   # HTTP
```

Exact service names depend on release name and chart version — copy from the wizard.

### 13.2 Quick validation without Epinio code changes

1. Enable Beyla for the `epinio` namespace (or annotate workloads per chart docs).
2. Generate traffic against the Epinio API / an app route.
3. Open **Application Observability** / **Traces Drilldown** and confirm services appear.

### 13.3 Longer-term Epinio instrumentation (follow-up, not required to close spike)

- Instrument `epinio-server` with OpenTelemetry Go SDK (HTTP server, staging jobs, Helm ops)
- Propagate workspace / app attributes as resource attributes
- Document whether CLI actions should emit telemetry (usually no)

Record in findings: *effort estimate* and *whether Beyla alone is enough for v1*.

---

## 14. Step 10 — Relate product telemetry (upgrade-responder)

Epinio already ships product telemetry via Upgrade Responder.

### 14.1 Local reference setup

Follow [`docs/howtos/upgrade-responder.md`](upgrade-responder.md):

```bash
make install-upgrade-responder
# patches DISABLE_TRACKING / UPGRADE_RESPONDER_ADDRESS as documented
```

Dashboard panels today (InfluxDB) include:

- Active node map (geo)
- Active node count
- Active nodes by Epinio version
- Active nodes by server version
- Kubernetes version / platform

### 14.2 Decision the spike should make

Pick one recommendation:

1. **Keep upgrade-responder Grafana separate**; Grafana Cloud is only for cluster/app ops observability.
2. **Dual-write / export** upgrade-responder metrics into Grafana Cloud Prometheus for a unified product + ops view.
3. **Replace** InfluxDB-backed dashboard with Cloud-native metrics (larger migration).

For each option, note: data ownership, privacy, retention, and whether Cloud is an appropriate home for **anonymous fleet telemetry**.

---

## 15. Step 11 — Document limitations

Fill this table during the spike (copy into Jira when done).

| Area | Finding | Severity | Mitigation / follow-up |
|------|---------|----------|------------------------|
| First-party OTel in Epinio | Not present | High for app RED dashboards | Beyla short-term; SDK later |
| Staging metrics | Mostly logs/events | Medium | Custom metrics or job scrapes |
| Upgrade-responder vs Cloud | Different backends (Influx vs Mimir) | Medium | Dual-write or keep split |
| Log volume / cost | Pod logs can dominate free tier | High | Namespace allowlists, drop debug |
| Cardinality | Per-pod / per-route labels | High | Label policy |
| PII / Confidential | Logs may contain user data | High | Scrubbing, namespace filters, private stacks |
| Multi-tenancy | Workspace isolation in dashboards | Medium | Variables + RBAC in Grafana |
| Auth to Cloud | Long-lived tokens | Medium | Short expiry, K8s secrets, rotation |
| Local clusters | k3d/kind quirks with node metrics | Low | Document platform choice |
| Billing | Host/container hours start when K8s monitoring enabled | Medium | Tear down after spike |

Also capture:

- Time to first useful dashboard
- Anything confusing in the Grafana wizard / Helm values
- Gaps vs competitor or Rancher Observability expectations (if relevant to EPINIO-681)

---

## 16. Step 12 — Spike exit criteria and findings template

### Exit criteria

- [ ] Grafana Cloud stack configured; recording rules installed
- [ ] Alloy (k8s-monitoring) running on a cluster with Epinio
- [ ] Metrics, logs, and events verified in Explore / Kubernetes Monitoring
- [ ] Built-in dashboards reviewed; gaps listed
- [ ] Proposed custom dashboard list agreed (even if only stubs exist)
- [ ] Upgrade-responder relationship decision recorded
- [ ] Limitations table completed
- [ ] Cleanup plan executed or scheduled
- [ ] Findings posted on EPINIO-685 (and linked from EPINIO-681)

### Findings comment template (paste into Jira)

```markdown
## EPINIO-685 spike findings

### Setup performed
- Grafana Cloud stack: <name>
- Cluster: <name / provider>
- Chart / method: k8s-monitoring <version> via <wizard|helm>
- Signals enabled: metrics / logs / events / otel / beyla

### What worked
- ...

### Useful dashboards
1. Built-in: ...
2. Custom proposed: Control plane / Staging / Apps / Fleet

### Limitations
- ...

### Recommendation
- [ ] Infra-only Grafana Cloud for Epinio ops
- [ ] Plus product telemetry migration from upgrade-responder
- [ ] Plus first-party OTel on epinio-server (follow-up epic)

### Follow-up tickets
- ...
```

---

## 17. Cleanup

When the spike ends:

```bash
helm uninstall grafana-k8s-monitoring -n monitoring
kubectl delete namespace monitoring
# revoke the Grafana Cloud access policy token in grafana.com
```

Also:

1. Remove unused dashboards or mark them as spike-only.
2. Delete or expire the Cloud stack if it was created only for this work.
3. Ensure no tokens remain in shell history notes shared with the team.

---

## 18. References

### Grafana Cloud

- [Kubernetes Monitoring Helm chart overview](https://grafana.com/docs/grafana-cloud/monitor-infrastructure/kubernetes-monitoring/configuration/helm-chart-config/helm-chart/)
- [Configuration steps (wizard)](https://grafana.com/docs/grafana-cloud/monitor-infrastructure/kubernetes-monitoring/configuration/helm-chart-config/hc-config-steps/)
- [OTel Collector alternative path](https://grafana.com/docs/grafana-cloud/observe-and-act/monitor-infrastructure/kubernetes-monitoring/configuration/config-other-methods/otel-collector/)
- [Access policies](https://grafana.com/docs/grafana-cloud/account-management/authentication-and-permissions/access-policies/)

### Epinio

- [`docs/howtos/upgrade-responder.md`](upgrade-responder.md) — product telemetry + local Grafana
- `helm-charts/chart/upgrade-responder` — InfluxDB + dashboard ConfigMap
- `docs/explanations/futureplans.md` — OpenTelemetry as future interest
- Public dashboard: https://version.rancher.io (Epinio Upgrade Checker)

### Ticket mapping

| ID | Title | This doc |
|----|-------|----------|
| EPINIO-681 | Parent epic | Link findings upward |
| EPINIO-685 | Define Grafana Cloud setup | Primary runbook |

---

## Appendix A — Minimal verification checklist (print / tick)

1. [ ] Backend alert + recording rules installed in Cloud  
2. [ ] Access token created and stored securely  
3. [ ] `helm upgrade --install` succeeded  
4. [ ] Alloy pods Running  
5. [ ] `kube_pod_info` for `namespace="epinio"` returns series  
6. [ ] Loki shows `epinio-server` logs  
7. [ ] Kubernetes Monitoring cluster status green  
8. [ ] At least one custom dashboard saved or explicitly deferred  
9. [ ] Limitations table filled  
10. [ ] Token revoked / env cleaned after spike  

## Appendix B — Common failures

| Symptom | Likely cause | What to try |
|---------|--------------|-------------|
| No metrics | Recording rules missing; wrong cluster label; auth failure | Re-install backend rules; check Alloy logs for `401`/`403` |
| No logs | Pod logs disabled; wrong Loki URL/user | Re-check wizard toggles; verify Loki credentials |
| Empty workload views | Backend rules not installed | Kubernetes Monitoring → install rules |
| Huge bill / quota | High log volume or cardinality | Drop noisy namespaces; disable debug logs; shorten retention |
| `ImagePullBackOff` on OpenCost/Kepler | Private registry / unused components | Disable cost/energy for spike |
| Connection test fails | Outbound network policy / proxy | Allow Alloy egress to `*.grafana.net` |

---

*Document status: spike runbook for EPINIO-685. Update this file with concrete URLs, chart versions, and findings as the spike progresses.*
