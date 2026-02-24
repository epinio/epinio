# How to test the Pack integration

This guide covers unit tests, manual end-to-end testing, and optional acceptance tests for the Pack-based staging path.

---

## 1. Unit tests

The Pack-related logic in `stage.go` is covered by unit tests in `internal/api/v1/application/stage_test.go`.

**Run only the Pack/staging helpers:**

```bash
cd epinio
go test ./internal/api/v1/application/ -run 'TestBuildContainerImage|TestAssembleStageEnvBUILDER_IMAGE|TestMountDockerSocket' -v
```

**Run all application package tests (including existing `jobDoneState` and the above):**

```bash
go test ./internal/api/v1/application/ -v
```

**Run with coverage:**

```bash
go test ./internal/api/v1/application/ -cover -coverprofile=coverage.out
go tool cover -func=coverage.out
```

These tests verify:

- `buildContainerImage()` uses `BuilderImage` when `BuildContainerImage` is empty, and uses `BuildContainerImage` when set (Pack path).
- `assembleStageEnv()` does not add `BUILDER_IMAGE` when not using Pack, and adds it with the correct builder image when using Pack.
- `mountDockerSocket()` adds the Docker socket volume and build-only mount only when both Pack build image and `dockerSocketPath` are set.

---

## 2. Manual end-to-end test (Pack path)

You need a cluster with Epinio installed with Pack staging enabled, and (if Pack must run the builder in a container) a Docker socket or DinD available to the staging job.

### 2.1 Build Epinio

```bash
cd epinio
make
# Or, if you use patch-epinio-deployment:
make && make patch-epinio-deployment
```

### 2.2 Install or upgrade Epinio with Pack enabled

Use the Epinio Helm chart with Pack values. From the **helm-charts** repo (or the chart in `epinio/helm-charts` if testing from the epinio tree):

```bash
# Example: install with Pack (set your domain and other required values)
helm upgrade --install epinio ./chart/epinio \
  --namespace epinio --create-namespace \
  --set global.domain=<your-domain> \
  --set server.stagingUsePack=true \
  --set server.pack.image=buildpacksio/pack:0.36.0
```

If Pack must run the builder in a container (e.g. on a node that has Docker), set the Docker socket path:

```bash
--set server.stagingWorkloads.dockerSocketPath=/var/run/docker.sock
```

(On some clusters you may need a node selector or DinD instead; see [Pack integration for staging](pack-integration.md).)

### 2.3 Push an app and confirm it uses Pack

1. Create a namespace and push a simple app (default builder is jammy, which will use the Pack script when `stagingUsePack` is true):

   ```bash
   epinio namespace create workspace
   epinio target workspace
   epinio push myapp --path <path-to-simple-app>
   ```

2. **Confirm the build used Pack:**
   - **Staging logs:**  
     `epinio app logs myapp --staging`  
     You should see Pack CLI output (e.g. `pack build ...`) instead of lifecycle/creator output.
   - **Staging job image:**  
     The build container image should be the Pack image (e.g. `buildpacksio/pack:0.36.0`), not the builder image:

     ```bash
     kubectl get jobs -n epinio -l app.kubernetes.io/component=staging -o wide
     kubectl get pods -n epinio -l app.kubernetes.io/component=staging -o jsonpath='{.items[*].spec.containers[?(@.name=="buildpack")].image}'
     ```

3. **Confirm the app runs:**  
   `epinio app list` and open the app URL; the app should be reachable and healthy.

### 2.4 Rollback test (optional)

1. Set Pack off and upgrade:

   ```bash
   helm upgrade epinio ./chart/epinio -n epinio --set server.stagingUsePack=false
   ```

2. Push again (or restage). Staging should use the lifecycle/creator path again (jammy script back), and `epinio app logs <app> --staging` should show lifecycle output, not Pack.

---

## 3. Acceptance tests (optional)

The standard acceptance suite runs with the default chart (Pack disabled). To test the Pack path in acceptance:

1. **Use a custom values file** that enables Pack (and optionally `dockerSocketPath`) and point your install/upgrade at it, if your test flow supports a values file.
2. **Run application-focused tests** after installing with Pack enabled:

   ```bash
   make test-acceptance-api-apps
   # or
   make test-acceptance-cli-apps
   ```

   These include push/stage/deploy; with Pack enabled they will use the Pack staging path.

3. If your cluster does not provide a Docker socket to the staging pod, Pack may fail when it tries to run the builder container. In that case either:
   - set `server.stagingWorkloads.dockerSocketPath` to a socket that is available (e.g. DinD), or  
   - run acceptance with Pack disabled and rely on unit + manual E2E for Pack.

---

## 4. Quick checklist

| What to test              | How |
|---------------------------|-----|
| Unit: build image choice  | `go test ./internal/api/v1/application/ -run TestBuildContainerImage -v` |
| Unit: BUILDER_IMAGE env   | `go test ./internal/api/v1/application/ -run TestAssembleStageEnvBUILDER_IMAGE -v` |
| Unit: Docker socket mount | `go test ./internal/api/v1/application/ -run TestMountDockerSocket -v` |
| E2E: Pack staging works   | Install with `stagingUsePack=true`, push app, check staging logs and build image |
| E2E: Rollback             | Set `stagingUsePack=false`, upgrade, push/restage and confirm lifecycle path |

For more on enabling Pack and required values, see [Pack integration for staging](pack-integration.md) (in the docs repo) or `docs/pack-integration-plan.md` in the epinio repo.

---

## 5. Troubleshooting "Failed to stage"

If push fails with "Failed to stage" and you only see download/unpack logs (no **buildpack** container output), the build step is failing.

**Staging jobs live in the Epinio system namespace** (e.g. `epinio`), not in the app namespace.

**Single command:** stream the **buildpack** container logs (Pack or lifecycle) for the first listed staging job. Replace `epinio` with your Epinio install namespace if different.

```bash
kubectl logs -n epinio -c buildpack -f --tail=50 $(kubectl get pods -n epinio -l "job-name=$(kubectl get jobs -n epinio -l app.kubernetes.io/component=staging -o jsonpath='{.items[0].metadata.name}')" -o jsonpath='{.items[0].metadata.name}')
```

Run this right after starting a push (or after a failure, if the job is still present). It picks the first staging job, finds its pod, and follows the buildpack container logs. If you get an error (e.g. no resources), the job may have been deleted by TTL—trigger a new push and run this command again as soon as the push starts so the job is still there.

**When the pod shows `ContainerCannotRun`:** the main container never started, so `kubectl logs` may be empty. The real reason is in the pod **events**. Use one of the following.

Describe the **first** staging job's pod (last 30 lines):

```bash
POD=$(kubectl get pods -n epinio -l "job-name=$(kubectl get jobs -n epinio -l app.kubernetes.io/component=staging -o jsonpath='{.items[0].metadata.name}')" -o jsonpath='{.items[0].metadata.name}') && kubectl describe pod -n epinio $POD | tail -30
```

Or set the job name and describe that job's pod (replace `JOBNAME` with the job from `kubectl get jobs -n epinio`):

```bash
JOB=JOBNAME; POD=$(kubectl get pods -n epinio -l "job-name=$JOB" -o jsonpath='{.items[0].metadata.name}') && kubectl describe pod -n epinio $POD | tail -30
```

Look for the **Events** section: common causes are image pull errors (e.g. `ErrImagePull`, `ImagePullBackOff`), volume mount failures, or security-context/runAsUser issues.

**Common causes:**

- **`ContainerCannotRun`:** The buildpack container never started. Run the `kubectl describe pod ...` command above and check **Events**. Common messages: `exec: "/bin/bash": no such file or directory` (Pack image has no bash—server must use `/bin/sh` for Pack; see below), image pull errors (`ErrImagePull`, `ImagePullBackOff`), volume mount failures, or security-context/runAsUser issues.

**If you still see `/bin/bash: no such file or directory` after `make patch-epinio-deployment`:**
1. Run `make` first (so the binary includes the `/bin/sh` fix), then `make patch-epinio-deployment`.
2. Wait for the server to roll out: `kubectl rollout status deployment -n epinio epinio-server`.
3. Confirm the server pod is new: `kubectl get pods -n epinio -l app.kubernetes.io/name=epinio-server` (pod AGE should be after the patch).
4. Delete the failed staging job so the next push creates a new one: `kubectl delete job -n epinio JOBNAME`.
5. Push again; the new job will be created by the updated server with `/bin/sh`.
- **Pack path:** `pack` not found in the image, or Pack needs a container runtime and no Docker socket is configured. Set `server.stagingWorkloads.dockerSocketPath` (e.g. `/var/run/docker.sock`) if the node has Docker, or disable Pack (`server.stagingUsePack: false`) to use the lifecycle path.
- **Lifecycle path:** Builder image pull or run failure; check registry access and that the default builder image (e.g. `paketobuildpacks/builder-jammy-full`) is pullable from the cluster.
- **Resources / security:** Build pod OOMKilled or RunAsUser/RunAsGroup issues; adjust `server.stagingWorkloads.resources` or builder user/group if needed.
