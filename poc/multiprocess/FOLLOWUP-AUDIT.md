# Follow-up audit: Epinio multi-process POC

Audit date: 2026-08-06 UTC

Source commit: `f84b932e866e442490cf134ef72e5d90adf8a01f`

## Verdict

The corrected POC demonstrates that one Epinio Helm release can render and coordinate web, worker, cron, and release processes from one prebuilt or Epinio-staged image. Failed release hooks prevent ordinary workloads from changing, and an unhealthy worker causes Helm atomic rollback to restore the prior Deployment and CronJob specifications.

The production assessment remains **feasible but invasive**. The Helm boundary works; Epinio's desired-state persistence, application-wide pod discovery, global scaling, status, logs, exec, restart, and missing one-off command API do not yet provide first-class process semantics.

This audit does not certify the POC as production-ready. In particular, migration side effects and CronJob executions are not rollbackable, Epinio's App CR diverges from the deployed Helm revision after failure, and the authoritative CRD module/schema was not versioned or migrated.

## Audit method

The audit treated the earlier README as unverified and inspected:

- the complete tracked and untracked git diff;
- App models, create/update/deploy handlers, staging and image selection, Helm value construction, workload discovery, status, logs, exec, scaling, restart, routes, services, and bindings;
- the live App CRD schema and AppChart object;
- the chart archive actually served by the cluster, not only the working-tree templates;
- rendered prebuilt, staged, and zero-replica manifests;
- Helm history, values, hooks, and failed revisions;
- live Deployments, ReplicaSets, Services, Ingresses, Certificates, CronJobs, Jobs, Pods, events, commands, images, replica counts, and logs;
- a clean seven-revision live test matrix and a separate rollback-convergence capture.

## Requirement and deliverable audit

| Requirement | Result | Direct evidence or qualification |
|---|---|---|
| Trace current Epinio architecture | Met | README architecture summary plus source locations listed below. |
| Add named web, worker, cron, and release processes | Met for POC | `models.go`, CRD patch, API persistence, Helm values, and chart templates. Exported live manifest contains all four definitions. |
| One coordinated application release | Met | All ordinary resources are owned by one hashed Helm release; history is in `evidence/live-audit3/*/helm-history.json`. Hook Jobs are Helm hooks and deliberately retained outside the ordinary manifest lifecycle. |
| Same staged image, different commands | Met | Revision 7 uses one runtime image in both Deployments, CronJob, and release Job; commands use the CNB launcher plus process-specific args. |
| Process-specific replicas | Met | v1 web/worker `2/2`; v2 and staged web `2/2`, worker `3/3`. Zero replicas is render/unit tested but was not deployed live. |
| Web Deployment + Service/route | Met after infrastructure follow-up | The matrix proved the Deployment, Service, TLS Certificate, Ingress, and Service response. After the baseline listeners moved from 8000/8443 to 80/443, direct standard-port HTTPS returned the staged web response after Cilium component restarts with no local bridge. |
| Worker Deployment | Met | Healthy and deliberately crashing worker revisions were deployed. Kubernetes events record all three v4 worker pods in `BackOff`. |
| Scheduler CronJob | Met | The Kubernetes CronJob controller created Jobs for revisions 1, 2, 5, and 7. Captured v1, v2, and staged logs report the expected versions. |
| Successful release/migration | Met | Revision-named hook Jobs for v1, v2, v4, and staged images completed and have retained logs. |
| Failed release blocks deployment | Met | Revision 3 failed with `BackoffLimitExceeded`; retained pod exit code is 42. Revision 4 rolled back to revision 2 without changing ordinary workloads. |
| Unhealthy process fails whole release | Met | Revision 5 timed out after 198.77s; Helm revision 6 rolled back. Captured events show all three crashing worker pods in BackOff. |
| Rollback restores every prior ordinary process/image/replica state | Met with qualification | The final harness asserted web `2/2` v2, worker `3/3` v2, CronJob v2, Service response v2, zero candidate Deployment pods, and zero nonzero candidate ReplicaSets before snapshotting. Independent convergence evidence records the same result. Hook Jobs, CronJob-created Jobs, and external migration effects are not rollback targets. |
| Services remain independent | Source-confirmed only | No service code was changed and service instances still use separate Helm releases. The chart mounts Epinio binding/configuration Secrets into each process pod. No live database/service instance was created in this follow-up. |
| One-off CLI commands | Not implemented | This is a missing first-class capability, not a passing deliverable. It requires a revision-aware ephemeral Job API/CLI. |
| Reproduction instructions | Met after correction | README uses repeatable install, static verification, and live audit scripts and explains the historical baseline port mismatch and its correction. |
| Existing assumptions needing work | Met | Status, readiness, logs, exec, scale, restart, routes, stage tracking, bindings, hooks, and desired/deployed state are documented. |
| Production effort and assessment | Met | README retains an 8-12 engineer-week estimate and “feasible but invasive” assessment. |

## Findings and fixes

### Correctness and evidence issues fixed

1. **The cluster served a stale chart.** The live ConfigMap still contained `replicas: {{ default 1 ... }}` while the working tree preserved zero correctly. `scripts/install-chart.sh` now packages the current chart, updates the ConfigMap, restarts the subPath-mounted chart server, updates the AppChart cache-busting URL, and compares package and ConfigMap SHA-256 values.

2. **The earlier staged success was overstated.** The original source push created a staged image but failed revisions 7 and 9; revision 11 succeeded only after `app restart`, leaving the App origin at v2. The clean audit performed a direct source push with the final chart. It succeeded as revision 7 and persisted the source path origin.

3. **Failed hook evidence was overwritten.** A constant hook Job name was deleted before rollback or the next upgrade. Hook Jobs now include `.Release.Revision`, retaining the failed revision-3 pod, log, and exit code.

4. **The routed chart omitted Epinio's normal TLS behavior.** The chart now renders a cert-manager Certificate when no matching route secret is provided and always configures Ingress TLS. The live Certificate is Ready.

5. **Staged-image detection used substring matching.** A prebuilt image could be misclassified if its path or tag happened to contain the stage ID. Detection now requires the complete image tag to equal the stage ID and has positive, negative, and empty-stage unit coverage. It remains a POC heuristic.

6. **Routes could have no process target.** Create/update validation now rejects external routes for a non-legacy process map unless one deployment has `routes: true`. Legacy process-less apps remain valid.

7. **Manifest/API round-trip coverage was incomplete.** Tests now cover manifest YAML decoding, update request propagation, CR decoding, validation, explicit zero replicas, multiple routed/release processes, and Helm values.

8. **There was no reproducible evidence harness.** Added `verify-static.sh`, `install-chart.sh`, and `run-live-audit.sh`. The live script refuses to overwrite an existing App and asserts state before writing each snapshot.

9. **Rollback convergence was initially recorded too early.** The first post-rollback snapshot contained terminating candidate web pods. The audit script now waits specifically for candidate Deployment pods and ReplicaSets to reach zero while excluding retained hook/Cron history. `evidence/rollback-converged/summary.txt` is the separate converged capture.

10. **The previous acceptance explanation was not directly captured.** A fresh API acceptance run failed in `SynchronizedBeforeSuite` with 404 on `/.well-known/openid-configuration` because Dex is disabled. It ran zero specs; the exact output is retained.

11. **Service and Ingress backend naming were not generated from one helper.** They now share the same bounded Service name. Chart `0.2.1` containing that fix was installed by checksum and used for the final seven-revision run.

12. **The live harness did not preserve all direct failure facts.** Every snapshot now captures application-related events, and a failed release hook must expose exit code 42 in terminated-container state before the test can pass.

13. **Composed resource names are not safe at maximum input lengths.** An exploratory `0.2.2` normal-name upgrade passed, but a 63-character application probe failed atomically because its CronJob name exceeded Kubernetes' 52-character limit. The incomplete hashed-name experiment was not retained in the final tree. Collision-safe, kind-specific naming remains a documented gap; evidence is in `evidence/chart-0.2.2-live/` and `evidence/long-name-live/push.log`.

### Newly established semantics

- Helm's automatic atomic rollback did **not** run the prior revision's release hook. Jobs `release-r4` and `release-r6` were not created.
- A successful candidate release Job and a candidate CronJob execution can remain after ordinary resources roll back.
- Epinio reports the staged application as `5/1`, lists historical hooks as instances, and showed an empty `Running StageId` because workload metadata is taken from the first application-wide pod.
- `epinio app logs` aggregates current processes and retained release history, including failed and superseded migrations.
- After failed revisions, `spec.imageurl` and `spec.processes` represent failed desired intent while `origin` and live Helm resources remain at the last successful v2 release.

## Commands and results

### Static and Go checks

```bash
poc/multiprocess/scripts/verify-static.sh poc/multiprocess/evidence/static
```

Result when run: pass. It runs focused Go tests, Helm lint, exact prebuilt/staged/zero-replica render assertions, Kubernetes client-side dry runs, and `git diff --check`. The retained `evidence/static/verification.log` was later overwritten by the experimental `0.2.2` naming run and is therefore not claimed as an exact final-tree rerun. The complete live matrix used final-tree chart `0.2.1`; after the naming experiment was reverted, only `git diff --check` was rerun at the user's direction.

```bash
go list ./... | rg -v '/acceptance($|/)' | xargs go test -count=1
```

Result: pass for every non-acceptance package. See `evidence/non-acceptance-go-test.log`.

```bash
go test ./acceptance/api/v1 -run TestAPI -count=1 \
  -ginkgo.label-filter=application
```

Result: fail before specs. OIDC discovery returned 404; 0 of 253 specs ran. See `evidence/acceptance/api-v1-blocker.log`.

### Live matrix

```bash
EPINIO_TEST_APP=multiprocess-audit3 \
EPINIO_EVIDENCE_DIR="$PWD/poc/multiprocess/evidence/live-audit3" \
  poc/multiprocess/scripts/run-live-audit.sh
```

Result: pass against chart `0.2.1`. The release was `multiprocess-2c659e0ad9a452a685ebfc317e797f78991ebeaf`.

| Revision | Helm status | Evidence |
|---:|---|---|
| 1 | superseded after later upgrade | v1 initial install; migration, scheduled cron, Service response, web `2/2`, worker `2/2`. |
| 2 | superseded | v2 upgrade; migration, scheduled cron, Service response, web `2/2`, worker `3/3`. |
| 3 | failed | Migration `fail=True`, exit 42, `BackoffLimitExceeded`. |
| 4 | superseded after later upgrade | Atomic rollback to revision 2; no rollback hook Job. |
| 5 | failed | v4 migration succeeded; worker pods entered BackOff; timeout. |
| 6 | superseded after staged upgrade | Atomic rollback to revision 4/v2 content; no rollback hook Job. |
| 7 | deployed | Direct staged push; source origin persisted; all process pods use stage `c52c51a39859a471`. |

The separate `multiprocess-audit` rollback capture ended at revision 6 and records fully converged v2 state in `evidence/rollback-converged/summary.txt`.

## Evidence map

- `evidence/live-audit3/summary.txt`: chart/revision history, exact timings, release exit code, BackOff events, stage ID, and revision-7 process state.
- `evidence/live-audit3/01-initial-v1/`: push log, revision history/values/manifest/hooks, full resources/events, migration log, controller-created cron log, and web response.
- `evidence/live-audit3/02-upgrade-v2/`: corresponding v2 evidence and replica change.
- `evidence/live-audit3/03-failed-release/`: failed values/hook, exit-code pod JSON, failed migration log, rollback history, and restored resources.
- `evidence/live-audit3/04-unhealthy-worker/`: failed values/manifest, v4 migration log, Kubernetes BackOff events, rollback history, candidate/rollback resources, and v2 web response.
- `evidence/live-audit3/05-staged-direct/`: direct build/push log, source-origin App CR, staged resources, migration/cron logs, and Service response.
- `evidence/live-audit3/exported-manifest.yml`: API/CLI manifest round trip containing all process definitions.
- `evidence/live-audit3/app-show.txt`: direct evidence of status and instance-model breakage.
- `evidence/live-audit3/app-logs.txt`: direct evidence of application-wide log aggregation.
- `evidence/rollback-converged/`: independent unhealthy rollback ending at the restored v2 release.
- `evidence/invalid-route-validation.log` and `evidence/invalid-route-update-validation.log`: live create/update rejection and preservation of the existing process map.
- `evidence/chart-0.2.2-live/`: successful normal-name live upgrade after the complete matrix; this experimental chart is not the final working-tree chart.
- `evidence/long-name-live/push.log`: atomic failure proving the unresolved CronJob 52-character name limit.

## Remaining gaps

- No first-class one-off command API or CLI.
- No process-aware status/readiness model, log selector, exec selector, scaling, or restart.
- No transactional desired/deployed revision state; a failed push leaves the App CR ahead of Helm.
- No rollback of migration side effects or CronJob-created work.
- No production CRD/API version migration, generated client/OpenAPI update, compatibility matrix, or upgrade tests.
- No live bound database/service test in this follow-up.
- Cilium ingress was validated after the audit rather than inside the seven-revision matrix; the matrix itself captured only Service traffic.
- No UI work, generalized multi-image model, per-process health schema, route-to-process mapping beyond one target, or hook retention policy.
- No collision-safe, per-kind resource naming for maximum-length application/process names; CronJobs require a 52-character bound.
- Acceptance specs remain blocked by this cluster's intentional Dex-disabled installation.

## Current cluster state

- `multiprocess-audit3`: Helm revision 8 deployed with experimental chart `0.2.2`, staged web `2/2`, worker `3/3`, CronJob active, Certificate Ready. The final working tree remains the fully matrix-tested chart `0.2.1` because the `0.2.2` long-name experiment was incomplete.
- `multiprocess-audit2`: earlier chart `0.2.0` audit release retained for comparison.
- `multiprocess-audit`: Helm revision 6 deployed at restored v2 state; its App CR still contains failed v4 desired intent for inspection.
- `multiprocess-poc`: original development release remains deployed for historical comparison.
- The 63-character long-name probe left an App CR with failed desired v1 intent; its atomic initial install was uninstalled and produced no successful application release.
- The Epinio server is the rebuilt `v0.0.0-dev` POC binary. The cluster's served chart and AppChart cache key remain experimental `0.2.2`/`?v=audit-3`; running `scripts/install-chart.sh` restores the final-tree `0.2.1`/`?v=audit-2` package.

All repository changes are intentionally uncommitted.

## Git status

```text
 M internal/api/v1/application/create.go
 M internal/api/v1/application/update.go
 M internal/api/v1/deploy/deploy.go
 M internal/application/application.go
 M internal/helm/helm.go
 M internal/helm/helm_test.go
 M internal/manifest/manifest_test.go
 M pkg/api/core/v1/models/models.go
 M pkg/api/core/v1/models/models_test.go
?? internal/application/processes.go
?? internal/application/processes_test.go
?? poc/
```
