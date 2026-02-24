# Plan: Integrate Pack with Epinio (End-to-End)

This document outlines a plan to integrate [Pack](https://buildpacks.io/docs/for-platform-operators/how-to/integrate-ci/pack/) (the CNB CLI) with Epinio so that application builds are driven by Pack instead of (or in addition to) the current direct CNB lifecycle invocation. The goal is to consolidate the developer experience and align with the standard buildpacks tooling.

---

## 1. Current State Summary

### 1.1 Build flow today

1. **CLI** (`epinio push`): User runs push with optional `--builder-image`. Source is uploaded to S3; API is called with `StageRequest{ App, BlobUID, BuilderImage }`.
2. **Server** (`internal/api/v1/application/stage.go`): Creates a Kubernetes **Job** with:
   - **Init containers**: Download blob from S3 → Unpack to `/workspace/source/app`.
   - **Build container**: Uses the **builder image** (e.g. `paketobuildpacks/builder-jammy-full`) and runs a script from a ConfigMap that invokes **`/cnb/lifecycle/creator`** with:
     - `-app=/workspace/source/app`, `-cache-dir=/workspace/cache`, `-layers=/layers`, `-platform=/workspace/source`, `-report=...`, `-previous-image=...`, `APPIMAGE`.
   - Registry credentials are mounted at `/home/cnb/.docker/`; the lifecycle pushes the built image directly (no Docker daemon in the pod).
3. **Deploy**: After the Job completes, deploy uses the image URL from the stage response.

### 1.2 Extension points

- **Staging scripts**: ConfigMaps with label `app.kubernetes.io/component: epinio-staging` define download/unpack/**build** scripts and optional `builder` glob (e.g. `*` or `paketobuildpacks/builder-jammy-*:*`). `DetermineStagingScripts()` picks a ConfigMap by builder image.
- **Build script**: The `build` key in the chosen ConfigMap is sourced in the build container. Current implementation runs the CNB **lifecycle binary** (`/cnb/lifecycle/creator`) directly.
- **Images**: Download image (e.g. awscli), Unpack image (bash/unpacker), **Builder image** (Paketo builder). The builder image is both the runtime for the build and the image that contains the lifecycle + buildpacks.

### 1.3 Important constraint

- The current design is **daemonless**: the lifecycle runs inside the builder container and pushes the app image using the registry config only (no Docker/containerd in the pod).
- **Pack CLI** typically runs the builder in a **container** (via Docker or containerd). So using Pack “as in the docs” implies having a container runtime available in the staging pod (e.g. Docker socket mount or DinD).

---

## 2. Goals and Non-Goals

**Goals:**

- Use Pack as the primary or optional path to build application source in Epinio.
- Preserve compatibility with existing `epinio push` and staging API (same `StageRequest`, same Job shape where possible).
- Align with [Pack documentation](https://buildpacks.io/docs/for-platform-operators/how-to/integrate-ci/pack/) so that Epinio’s behavior is consistent with “run `pack build` in CI”.

**Non-Goals (for this plan):**

- Removing the existing lifecycle-based path in the first phase (can be deprecated later).
- Changing the deploy or registry model.

---

## 3. High-Level Approach

**Recommended approach: Pack-based staging as an alternative build path**

- Introduce a **Pack-based staging path** that runs the **Pack CLI** inside the staging Job (instead of invoking `/cnb/lifecycle/creator`).
- Use an image that includes Pack (e.g. `buildpacksio/pack` or a custom image with Pack + Docker client) and, where needed, provide a container runtime (Docker socket or DinD) so `pack build` can run the builder container.
- Keep the same Job structure (init containers for download/unpack, one main container for build). Only the build container image and the build script change when “Pack” is selected.
- Selection of “Pack vs lifecycle” can be:
  - **Option A**: Configurable (e.g. server flag or staging script selection), defaulting to Pack for new installs once stable.
  - **Option B**: Always use Pack and remove the old script path after migration.

**Alternative (larger change):** Use the **Pack Go library** inside the Epinio server (or a dedicated build service) to perform builds, with the server having access to a container runtime. This would move build execution out of the current Job-based model and is out of scope for the initial integration.

---

## 4. Implementation Plan (End-to-End)

### Phase 1: Staging script and image for Pack

1. **New stage script ConfigMap (Pack)**  
   - Add a new template (e.g. `stage-scripts-pack.yaml`) that defines a build script that runs **`pack build`** instead of `/cnb/lifecycle/creator`.  
   - Script must:
     - Use `APPIMAGE`, `PREIMAGE` (for `--previous-image` if supported), `USERID`/`GROUPID` if relevant, and app path `/workspace/source/app`.
     - Pass builder image (e.g. from env `BUILDER_IMAGE` or existing builder selection).
     - Use cache dir `/workspace/cache` if Pack supports it (`--cache-dir` or volume).
     - Ensure registry credentials are used (e.g. `DOCKER_CONFIG=/home/cnb/.docker` or equivalent so Pack pushes to the Epinio registry).
   - Set `builder` pattern so this ConfigMap is chosen when a “Pack” builder/image is selected (or when a global “use Pack” flag is set—see Phase 2).

2. **Build container image for Pack**  
   - Current build container **is** the builder image (Paketo). For Pack, the build container should be an image that has:
     - **Pack CLI** (e.g. `buildpacksio/pack` or image derived from it).
     - **Docker client** and access to a container runtime (Docker socket or containerd).
   - Options:
     - Use **`buildpacksio/pack`** and mount the host’s Docker socket (or a DinD sidecar’s socket) into the build container.
     - Or build a **custom image** (e.g. Pack + Docker client + optional entrypoint) and document that Epinio’s Pack-based staging requires a container runtime (socket or DinD).
   - The **builder image** (e.g. Paketo) is then passed to `pack build --builder <builder-image>`; Pack will pull and run that image as the builder container.

3. **Helm values**  
   - Add values for:
     - Pack-based build: enable/disable, and (optional) image for the Pack build container if different from `buildpacksio/pack`.
     - Docker socket path or DinD configuration if needed (e.g. for default installs that use Pack).

4. **Job spec changes**  
   - When the Pack staging script is selected:
     - Use the **Pack build image** (not the builder image) as the main container image.
     - Pass builder image via env (e.g. `BUILDER_IMAGE`) so the script can run `pack build ... --builder $BUILDER_IMAGE`.
     - Mount registry credentials and (if required) Docker socket or DinD socket.
     - Keep the same volumes (source, cache, registry-creds, app-environment, staging scripts).  
   - This may require a small branch in `stage.go`: when the selected script config is “Pack”, set container image and env from new Helm/config (e.g. `config.PackBuildImage`, `config.UsePack`).

### Phase 2: Selection logic and API

5. **When to use Pack**  
   - **Option A – Script-only:** Add a new staging script ConfigMap (e.g. `epinio-stage-scripts-pack`) with a `builder` pattern that matches a specific image (e.g. `paketobuildpacks/builder*`). Then “using Pack” is just “using that script” when the user (or default) selects a builder that matches. No API change.  
   - **Option B – Explicit toggle:** Add a server/Helm option (e.g. `server.stagingUsePack: true`) and, when true, prefer a Pack script (or a dedicated Pack ConfigMap) regardless of builder.  
   - **Option C – Request-level:** Add an optional field on `StageRequest` (e.g. `UsePack bool`) and/or manifest (e.g. `staging.usePack`) so the user can opt in per push.  
   - Recommendation: Start with **Option A** (new Pack script + image, selected by builder pattern or a single “use Pack” Helm value) to avoid API churn. Option B/C can be added later.

6. **Builder image**  
   - Keep current behavior: builder image comes from request, app spec, or default. For Pack, that image is passed to `pack build --builder <image>`.

7. **Backward compatibility**  
   - Keep existing `epinio-stage-scripts` (and jammy/bionic) so that installs that do not enable Pack continue to use the lifecycle/creator path. No change to `StageRequest` contract in Phase 1.

### Phase 3: CLI and manifest

8. **CLI**  
   - No strict requirement to expose “use Pack” in the CLI initially if selection is via server/Helm or builder pattern.  
   - If you add `staging.usePack` in the manifest, document it and ensure `epinio push` passes it through (e.g. in the stage request or in a future field).

9. **Manifest**  
   - Optional: add `staging.usePack` and/or `staging.packImage` in the application manifest for overrides. Server would need to support these (Phase 2 Option C).

### Phase 4: Testing and documentation

10. **Tests**  
    - Unit: Script selection returns Pack config when appropriate (builder pattern or flag).  
    - Acceptance: Push an app with Pack-based staging enabled and assert the app builds and deploys (same as existing staging tests but with Pack path).  
    - Optional: Test with Docker socket vs DinD if both are supported.

11. **Documentation**  
    - Document that Epinio can use [Pack](https://buildpacks.io/docs/for-platform-operators/how-to/integrate-ci/pack/) for building application source.  
    - Document Helm values (Pack image, Docker socket/DinD, enable/disable).  
    - Document any security considerations (Docker socket access, optional root/DinD).

12. **Release notes**  
    - Describe Pack integration as an alternative or default build path and how to enable it.

---

## 5. Technical Details to Resolve During Implementation

- **Environment variables**: Map existing env (e.g. `CNB_PLATFORM_API`, user env from app) into the Pack build script (e.g. `--env` or buildpacks env files).  
- **Cache**: Confirm Pack’s `--cache-dir` or volume mapping matches `/workspace/cache` and that the same PVC/emptyDir is used so cache is reused across builds.  
- **Previous image**: Use `--previous-image` (or equivalent) in `pack build` for layer reuse if supported.  
- **Registry**: Ensure `DOCKER_CONFIG` (or equivalent) points at the mounted registry secret so Pack pushes to the Epinio registry.  
- **Resource limits**: Pack may run additional containers; consider slightly higher defaults for the build pod when Pack is used.  
- **Security**: Document and harden Docker socket mount (or DinD) if used in production; consider node selector / taints for build nodes.

---

## 6. Rollout and Rollback

- **Rollout**: Ship Pack script and image behind a feature flag or builder pattern; enable by default only after testing.  
- **Rollback**: Disable Pack (e.g. remove or don’t install Pack ConfigMap, or set `stagingUsePack: false`); staging falls back to existing lifecycle script.  
- **Deprecation**: Once Pack is default and stable, deprecate the direct lifecycle/creator script and eventually remove it in a later major version.

---

## 7. Summary Checklist

| Area              | Action |
|-------------------|--------|
| Stage scripts     | Add `stage-scripts-pack.yaml` with `pack build` script; set `builder` pattern and env. |
| Build image       | Use or add image with Pack CLI + Docker client; document runtime (socket/DinD). |
| Helm              | Add values for Pack build image, enable/disable, optional socket/DinD. |
| Job spec          | When Pack script selected: use Pack image, pass `BUILDER_IMAGE`, mount socket if needed. |
| Selection         | Prefer Pack script by builder pattern or server/Helm flag (Option A/B). |
| API/CLI           | Optional: `UsePack` or manifest fields later (Option C). |
| Tests             | Unit for script selection; acceptance for push with Pack. |
| Docs              | Pack integration, Helm options, security note. |

This plan gets Pack integrated end-to-end while keeping the existing lifecycle path available and the same overall push → stage → deploy flow.
