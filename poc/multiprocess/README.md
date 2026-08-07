# Epinio multi-process POC

This POC adds a deliberately small, typed process model to Epinio and passes it to a custom application chart. The live tests were run on 2026-08-06 against:

- Epinio source commit `f84b932e866e442490cf134ef72e5d90adf8a01f` (2026-07-30)
- Epinio Helm chart `1.14.1`
- k3s/Kubernetes `v1.36.3+k3s1`
- Helm `v3.21.3`
- cert-manager `v1.21.1`

The result is **feasible but invasive**. Rendering and coordinating multiple workloads in one Helm release is a clean fit. Epinio's management APIs and workload model, however, assume one scalable process strongly enough that a production implementation requires meaningful cross-cutting work.

## Current Epinio deployment path

1. `epinio push` creates or updates the passive `application.epinio.io/v1 App` CR. Environment, scale, configuration bindings, and service bindings are stored in associated Secrets.
2. Source pushes upload an archive to Epinio's S3 store. A staging Job runs Cloud Native Buildpacks and pushes one image to the registry. The App CR records `stageid`, builder, blob ID, and image URL.
3. The deploy endpoint writes `spec.imageurl` before attempting the workload deployment, then `internal/api/v1/deploy` loads the App CR, scale Secret, environment, bindings, routes, domains, and selected AppChart.
4. `internal/helm.getValuesYAML` constructs the chart contract under `epinio.*`, plus operator `chartConfig` and validated `userConfig`.
5. Epinio installs or upgrades one hashed Helm release with `Wait: true`, `Atomic: true`, `ReuseValues: true`, and a three-minute deployment timeout.
6. Only after Helm succeeds does Epinio persist the new source origin and clean older staging artifacts.
7. Status, logs, and exec discover pods by application-wide labels. Scale remains a single integer in one Secret. Services use separate service Helm releases; bindings feed configuration Secrets into the app release.

## POC model and resources

The manifest/API model accepts:

```yaml
configuration:
  processes:
    web:
      kind: deployment
      command: ["python", "app.py", "web"]
      replicas: 2
      routes: true
    worker:
      kind: deployment
      command: ["python", "app.py", "worker"]
      replicas: 3
    scheduler:
      kind: cron
      command: ["python", "app.py", "cron"]
      schedule: "* * * * *"
    release:
      kind: release
      command: ["python", "app.py", "migrate"]
```

The POC persists this under `App.spec.processes`, returns it in application manifests/API responses, and passes it as structured `epinio.processes` Helm values.

The chart renders:

- `deployment` -> one Deployment per named process
- the one deployment with `routes: true` -> Service, TLS Certificate, and application Ingress
- `cron` -> CronJob
- `release` -> a revision-named `pre-install,pre-upgrade` Helm hook Job with `backoffLimit: 0`

Every process pod receives the same `epinio.imageURL`, environment, stage ID, configuration mounts, and application identity labels. Process selectors add `epinio.io/process-name` so Deployments do not overlap. Resources also carry `epinio.io/release-revision` for auditability.

For an Epinio-staged CNB image whose complete image tag equals the stage ID, Epinio adds `epinio.staged: true`. The chart explicitly invokes `/cnb/lifecycle/launcher -- <command...>` so buildpack `exec.d` environment setup is retained. For a prebuilt container image, the chart uses the process command directly. This is still an execution-mode heuristic and should become explicit API state in production.

## Code layout

- Core model/API/CR plumbing: `pkg/api/core/v1/models/models.go`, `internal/application/application.go`, `internal/application/processes.go`, and the create/update/deploy handlers.
- Helm contract: `internal/helm/helm.go`.
- CRD schema POC patch: `crd-processes-json-patch.json`.
- Application chart and chart server: `chart/`, `chart-server.yaml`, and `appchart.yaml`.
- Test app and manifests: `test-app/` and the `epinio-*.yml` files.
- Repeatable checks: `scripts/verify-static.sh`, `scripts/install-chart.sh`, and `scripts/run-live-audit.sh`.
- Captured outputs: `evidence/static/`, `evidence/live-audit3/`, `evidence/rollback-converged/`, and `evidence/acceptance/`.

The upstream App CRD types live in the separate `github.com/epinio/application` module, which this checkout pins to a 2023 pseudo-version. The POC avoids changing that dependency and supplies an explicit CRD JSON patch. A production change must update the authoritative CRD/type source and add conversion and upgrade coverage.

## Reproduction

The commands below assume a working Kubernetes context, Docker, Helm, and the Epinio source checkout. Replace the domain/IP for another cluster.

Install cert-manager first so its CRDs exist before the Epinio and POC Certificate resources are applied:

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo add epinio https://epinio.github.io/helm-charts
helm repo update

helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true --wait

helm upgrade --install epinio epinio/epinio \
  --namespace epinio --create-namespace \
  --set global.domain=10.42.0.42.sslip.io \
  --set certManager.install=false \
  --set ingress.ingressClassName=cilium \
  --set server.ingressClassName=cilium \
  --set global.dex.enabled=false \
  --wait --timeout 12m
```

Build and install the modified server binary and CRD schema:

```bash
kubectl patch crd apps.application.epinio.io --type=json \
  --patch-file=poc/multiprocess/crd-processes-json-patch.json

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -o dist/epinio-linux-amd64 .

EPINIO_BINARY_TAG=multiprocess-poc \
EPINIO_BINARY_PATH=dist/epinio-linux-amd64 \
EPINIO_NAMESPACE=epinio \
  ./scripts/patch-epinio-deployment.sh
```

Package and expose the custom AppChart to the Epinio API pod. The script restarts the chart server after updating its ConfigMap and verifies that the cluster archive checksum matches the package; both steps prevent testing a stale subPath-mounted chart:

```bash
poc/multiprocess/scripts/install-chart.sh
```

For the prebuilt-image cases, build and load all tags into the single-node k3s image store:

```bash
for version in v1 v2 v3 v4; do
  docker build \
    -t docker.io/epinio-poc/multiprocess:$version \
    --build-arg IMAGE_VERSION=$version \
    poc/multiprocess/test-app
done

docker save \
  docker.io/epinio-poc/multiprocess:v1 \
  docker.io/epinio-poc/multiprocess:v2 \
  docker.io/epinio-poc/multiprocess:v3 \
  docker.io/epinio-poc/multiprocess:v4 \
  | sudo k3s ctr images import -
```

On this VM, Cilium's shared ingress Service is ClusterIP-only and stopped being reachable from the host after a Cilium restart. The audited workaround uses a documented Kubernetes Service port-forward to the Epinio API and a local TLS bridge using Epinio's own certificate. Start the port-forward in one terminal:

```bash
kubectl -n epinio port-forward --address 127.0.0.1 \
  service/epinio-server 18081:80
```

In another terminal, create a temporary combined PEM without printing its contents and start the TLS bridge:

```bash
umask 077
kubectl -n epinio get secret epinio-tls \
  -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/epinio-poc-tls.pem
kubectl -n epinio get secret epinio-tls \
  -o jsonpath='{.data.tls\.key}' | base64 -d >> /tmp/epinio-poc-tls.pem

sudo socat \
  OPENSSL-LISTEN:443,bind=10.42.0.42,reuseaddr,fork,cert=/tmp/epinio-poc-tls.pem,verify=0 \
  TCP:127.0.0.1:18081
```

This bridge is only for the host CLI; it is not an Epinio or Kubernetes component. Restart the port-forward if the Epinio server pod is replaced. If the cluster already exposes its ingress normally, omit both commands.

Log in and select a workspace:

```bash
./dist/epinio-linux-amd64 login -u admin -p password --trust-ca \
  https://epinio.10.42.0.42.sslip.io
./dist/epinio-linux-amd64 namespace create poc
./dist/epinio-linux-amd64 target poc
```

Run the static checks and the complete live matrix:

```bash
poc/multiprocess/scripts/verify-static.sh poc/multiprocess/evidence/static

EPINIO_TEST_APP=multiprocess-audit3 \
EPINIO_EVIDENCE_DIR="$PWD/poc/multiprocess/evidence/live-audit3" \
  poc/multiprocess/scripts/run-live-audit.sh
```

The live script deliberately refuses to overwrite an existing test App. It verifies every assertion before recording a case snapshot. The four prebuilt image tags must already be present on the node as shown above.

## Live test results

The final complete matrix used chart `0.2.1`, a fresh application named `multiprocess-audit3`, and Helm release `multiprocess-2c659e0ad9a452a685ebfc317e797f78991ebeaf`. The full captured record is under `evidence/live-audit3/`; `summary.txt` records the exact history, timings, failed container exit code, BackOff events, stage ID, and revision-7 resources.

| Case | Observed result |
|---|---|
| Initial v1 | Revision 1 installed in 16.43s. Web `2/2`, worker `2/2`, migration Job completed with v1, a controller-created CronJob Job printed v1, and a Service port-forward returned v1. |
| Upgrade v2 | Revision 2 completed in 18.20s. Web stayed `2/2`, worker became `3/3`, migration and a new controller-created cron Job printed v2, and Service traffic returned v2. |
| Failed migration | Revision 3 returned 255 in 12.14s with `BackoffLimitExceeded`. The retained migration pod logged `fail=True`; the harness read exit code 42 from its terminated container. Atomic revision 4 rolled back to revision 2; web `2/2`, worker `3/3`, CronJob image/command/schedule, Service, Ingress, and Certificate remained/restored to v2 state. |
| Unhealthy worker | Revision 5's migration succeeded with v4. Captured Kubernetes events record all three v4 worker pods in `BackOff`; a v4 CronJob-created Job also completed while the candidate was pending. The push returned 255 after 198.77s and revision 6 rolled back to revision 4. Before snapshotting, the harness asserted zero remaining v4 Deployment pods and zero nonzero v4 ReplicaSets, plus web `2/2`, worker `3/3`, CronJob v2, and Service traffic v2. |
| Direct staged push | A single source `push` built stage `c52c51a39859a471` and deployed revision 7 in 54.09s. The App origin changed to the source path. Web `2/2`, worker `3/3`, CronJob, migration Job, and a controller-created cron Job used the same internal image and `/cnb/lifecycle/launcher`; all outputs reported `staged-v5`. |

The final staged command is `/cnb/lifecycle/launcher` with `--` plus the declared process command in `args`. Earlier development revisions 7 and 9 of the original `multiprocess-poc` release failed with two incomplete CNB command strategies; their Helm hook manifests remain inspectable, but the deleted pod logs do not. They are therefore development history, not part of the passing evidence matrix.

After that matrix, an experimental chart `0.2.2` added hashed overlength resource names. A normal-name live upgrade to revision 8 passed in 13.85s, including the release hook, web/worker replicas, CronJob, Certificate, and Service response (`evidence/chart-0.2.2-live/`). A separate 63-character application-name probe then failed atomically because Kubernetes limits CronJob names to 52 characters (`evidence/long-name-live/push.log`). The incomplete long-name change is not in the final working tree; chart `0.2.1` remains the reproducible POC and long process/application-name collision handling is an explicit gap. The cluster still contains the experimental revision 8, and the chart server still serves the experimental `0.2.2` archive, for inspection.

## Verification

All non-acceptance Go packages pass:

```bash
go list ./... | rg -v '/acceptance($|/)' | xargs go test -count=1
```

`scripts/verify-static.sh` passed focused Go tests, Helm lint, exact prebuilt/staged resource assertions, Kubernetes client-side dry runs, and `git diff --check`; rendering explicitly covered `replicas: 0`. The retained `evidence/static/verification.log` was subsequently overwritten by the experimental `0.2.2` naming run, so it is not claimed as an exact final-tree rerun. The application Go code did not change afterward, the complete live matrix used the final `0.2.1` chart, and the final post-reconciliation check was limited to `git diff --check` at the user's direction. The broader Go run is in `evidence/non-acceptance-go-test.log`.

At audit completion, `multiprocess-audit3` revision 8 remained deployed with the experimental chart `0.2.2`, staged web `2/2`, worker `3/3`, a staged CronJob, successful staged migration Job, ready Certificate, and TLS Ingress. The complete behavioral matrix immediately preceding it used the final-tree chart `0.2.1`. Application traffic was proven through a Kubernetes Service port-forward. End-to-end traffic through Cilium ingress from the VM was not proven because the ClusterIP-only ingress path was unavailable after Cilium restarted; this is an infrastructure limitation and the route claim is limited accordingly.

The API acceptance suite was rerun with its application label filter. Its `SynchronizedBeforeSuite` received 404 for `/.well-known/openid-configuration` because this cluster deliberately has Dex/OIDC disabled. It ran 0 of 253 specs. The exact output is `evidence/acceptance/api-v1-blocker.log`; no acceptance result is claimed.

### Atomicity qualifications

- A failed pre-upgrade migration hook runs before normal resources change, so the old release stays intact.
- If the release hook succeeds and a Deployment later fails, Helm rolls back Kubernetes release resources, but it cannot undo the migration's external effects.
- Automatic atomic rollback did not execute the prior revision's release hook: Jobs `release-r4` and `release-r6` were not created. This avoids rerunning the old migration, but rollback migrations are likewise not available.
- Hook Jobs and CronJob-created Jobs are history, not ordinary rollback targets. The successful v4 migration Job and a completed v4 scheduled Job remained after ordinary resources rolled back to v2.
- Helm returned rollback success while candidate pods were still terminating. The separate `evidence/rollback-converged/summary.txt` capture waited until candidate Deployment pods and ReplicaSet counts were zero. Atomicity does not mean simultaneous pod replacement.
- Most importantly, Epinio writes process configuration and `spec.imageurl` before Helm. After both failed upgrades, the App CR pointed at the failed v3/v4 intent while the live Helm release was v2. `origin` remained v2 because Epinio only updates it after success. A production design needs desired-vs-deployed revisions or transactional reconciliation.

## Existing assumptions that break

### Status and readiness

Epinio aggregates all pods labeled as the application and compares ready pods with one global desired instance count. The audited staged app showed `5/1`, while historical hook and current cron pods appeared as non-ready entries in the instance table. Because workload metadata comes from the first unsorted application pod, `Running StageId` was blank even though all current Deployments used the staged image. Status needs a process-indexed workload model and per-kind readiness and revision rules. Helm waits for Deployments and hook Jobs, but not for a CronJob to execute.

### Logs

Existing logs aggregate web, worker, cron, and every retained release hook, including failed and superseded revisions. `evidence/live-audit3/app-logs.txt` directly contains all four process categories. There is no process selector, so production UX needs `--process`, sensible defaults, bounded history, and explicit job handling.

### Exec and one-off commands

Exec selects from one application-wide pod list and derives a container name from application workload data. Without `--instance`, it uses the first unsorted pod and can choose the wrong process or a completed Job. `--instance` can target an exact pod, but there is no stable process selector. The POC uses the same container name in every pod only to avoid an additional failure mode.

One-off commands are not implemented as a first-class API in this POC. They need an endpoint/CLI that creates an ephemeral Job from a specific deployed application revision, uses the same bindings and CNB launcher semantics, streams exit status/logs, and applies retention policy. Existing interactive exec is not an adequate substitute.

### Scaling

Scale is one integer in one Secret and the CLI/API accept only application-wide `instances`. The POC chart intentionally uses per-process replicas, so the old scale value cannot represent or update them. Production needs `scale APP --process worker N`, per-process persistence, events, validation, and a backward-compatible default-web mapping.

### Restart

Restart redeploys the whole Helm release and cannot target a process. More seriously, after atomic rollback it reads the failed desired image/processes from the App CR and can retry the broken release. Restart must distinguish desired and last-deployed revisions.

### Routes

Current application routes converge on one Service. The POC permits exactly one routed deployment and now rejects external routes when a non-legacy process map has no routed target. Multiple independently routed web processes would require route-to-process mapping and collision validation. The audit verified Service traffic and ready TLS resources, not VM-to-Cilium ingress traffic.

### Stage/image tracking

One staged image and stage ID work cleanly for all processes. The audit also exposed that application-wide pod discovery can report the stage ID from an old hook pod. The CNB launcher distinction and deployed revision must become explicit execution state rather than tag and pod-order heuristics. Multi-image composition was intentionally out of scope.

### Service bindings

Source inspection confirms service instances remain separate Helm releases, and the chart mounts Epinio's binding/configuration Secrets into every process pod. The follow-up live matrix did not create a PostgreSQL/Redis-style service instance, so no live service-lifecycle claim is made. Production may want process-specific binding visibility, but services should remain independent from application rollback.

### Release jobs

The Helm hook is a good POC fit because failure blocks install/upgrade immediately. Revision-named Jobs retain direct success/failure evidence, but also worsen application-wide status/log noise and survive as hook history. Production work must define retries, timeouts, deletion/history, concurrent deploy behavior, idempotency, hook log retention, and whether rollback migrations are ever supported.

### Resource naming

The POC validates process names as DNS labels but does not yet define collision-safe composed names for maximum-length application/process pairs. Kubernetes also imposes a stricter 52-character limit on CronJob names. The failed long-name probe proves this is unresolved; production naming needs deterministic per-kind length bounds and hash suffixes with dedicated tests.

## Production effort estimate

For one engineer familiar with Epinio: roughly **8-12 engineering weeks** to reach a maintainable backend/CLI implementation, excluding polished UI work. A realistic breakdown is:

- 1-2 weeks: versioned schema/CRD migration, validation, desired/deployed revision model
- 2-3 weeks: production chart contract, hooks, CNB/container execution semantics, upgrades
- 2-3 weeks: process-aware status/workload inventory and reconciliation
- 2-3 weeks: logs, exec, scale, restart, routes, and one-off CLI/API behavior
- 1-2 weeks: acceptance tests, compatibility/upgrade coverage, docs and operational hardening

Two engineers could likely deliver a reviewed first production slice in 5-7 calendar weeks, with follow-up hardening.

## Assessment

**Feasible but invasive.** Epinio's Helm/AppChart boundary is an excellent extension point: one application release can naturally render multiple resources, successful upgrades are coordinated, hook failures block deployment, and atomic rollback restores ordinary workloads together. The difficult work is above and beside Helm—Epinio's App CR state transitions, global scale/status model, pod discovery, and process-agnostic operational commands. This is not fighting the fundamental deployment architecture, but it is substantially more than a chart-only feature.
