# How to Fix the gosec Lint Issues

This document explains each gosec finding from `make lint` and how to fix or justify it.

---

## 1. G204 – Subprocess launched with variable

**Where:** `acceptance/helpers/proc/proc.go` (lines 32, 44)

**Why gosec complains:** `exec.Command(command, args...)` with variable command/args can run arbitrary commands if input is untrusted.

**How to fix:**

- **Acceptance tests:** The command and args are controlled by test code, not user input. Add an inline nolint with a short reason:

```go
// Line 32:
p := exec.Command(command, arg...) // nolint:gosec // acceptance test helper, command from test code

// Line 44:
cmd := exec.Command(command, args...) // nolint:gosec // acceptance test helper, command from test code
```

---

## 2. G703 – Path traversal / taint (file operations)

**Where:** `internal/api/v1/application/export.go`  
- 445: `os.RemoveAll(path)`  
- 486: `os.Open(chartArchive)`  
- 718: `os.WriteFile(certFile, ...)`

**Why gosec complains:** Paths might come from user input and escape intended directories.

**How to fix:**

- **445 – cleanupLocalPath:** `path` is built from `imageExportVolume` + a destination path. Ensure callers only pass paths under a known temp/export root, and add a comment + nolint:

```go
err := os.RemoveAll(path) // nolint:gosec // path under imageExportVolume from internal export flow
```

- **486 – fetchAppChartFile:** `chartArchive` comes from urlcache (after URL→local path resolution). If urlcache is trusted and scoped, document and nolint:

```go
file, err := os.Open(chartArchive) // nolint:gosec // path from urlcache under controlled export volume
```

- **718 – loadCerts:** `certFile` is built as `imageExportVolume + "<prefix>-<nanosecond>.pem"`. So it’s under a fixed prefix. Nolint with reason:

```go
err = os.WriteFile(certFile, pemData, 0600) // nolint:gosec // certFile under imageExportVolume, name from time
```

**Stronger fix (optional):** For any of these, you can add explicit checks that the resolved path is under the allowed base (e.g. `filepath.Clean` + `strings.HasPrefix(clean, base)` or `filepath.Rel(base, clean)` and reject `..`).

---

## 3. G704 – SSRF (HTTP client / proxy)

**Where:**  
- `internal/api/v1/proxy/proxy.go:151` – `p.ServeHTTP(rw, req)`  
- `internal/registry/registry.go:452` – `client.Do(req)`  
- `internal/registry/registry.go:537` – `client.Do(listReq)`

**Why gosec complains:** Outgoing HTTP requests might be driven by user-controlled URLs (SSRF).

**How to fix:**

- **proxy.go:** The reverse proxy is designed to forward requests (e.g. port-forward). The target is controlled by cluster/service configuration, not arbitrary user URL. Nolint:

```go
p.ServeHTTP(rw, req) // nolint:gosec // reverse proxy for port-forward, target from cluster config
```

- **registry.go (452, 537):** Registry URLs come from cluster/registry configuration; requests are to the configured registry. Nolint:

```go
resp, err := client.Do(req) // nolint:gosec // registry URL from cluster config, not user input
// and
listResp, err := client.Do(listReq) // nolint:gosec // registry URL from cluster config, not user input
```

---

## 4. G117 – Exported struct field matches “secret” pattern

**Where:**  
- `internal/auth/user.go:33` – `Password`  
- `internal/bridge/git/git.go:55` – `Password`  
- `internal/cli/settings/settings.go:58,60` – `AccessToken`, `RefreshToken`  
- `internal/helm/helm.go:325` – `Secret`  
- `internal/registry/registry.go:47,53` – `Password`  
- `pkg/api/core/v1/models/gitconfig.go:28` – `Password`

**Why gosec complains:** Exported fields named like secrets might be logged or serialized by mistake.

**How to fix:**  
These are intentional: structs for auth, config, or API models. Renaming would break JSON/config and is a large change. Per-field nolint is the usual approach:

```go
Password string `json:"password"` // nolint:gosec // intentional auth field, not logged
```

Apply the same idea to each reported field (one nolint per line gosec points to). Optionally add a single file-level comment at the top of the file, e.g. “Auth/credentials structs, field names intentional.”

---

## 5. G705 – XSS via taint (fmt.Fprintf to stderr)

**Where:** `pkg/api/core/v1/client/http.go:329, 330, 331`

**Why gosec complains:** Response data is passed to `fmt.Fprintf(os.Stderr, ...)` and gosec treats it as possible XSS.

**How to fix:** Output goes to stderr for debugging, not into HTML. Nolint each line (or the block) with a short reason:

```go
fmt.Fprintf(os.Stderr, "URL: %s\n", response.Request.URL.String())       // nolint:gosec // debug to stderr, not HTML
fmt.Fprintf(os.Stderr, "Status: %d %s\n", response.StatusCode, response.Status) // nolint:gosec // debug to stderr
fmt.Fprintf(os.Stderr, "Content-Type: %s\n", response.Header.Get("Content-Type")) // nolint:gosec // debug to stderr
```

---

## Summary

| Rule | Count | Approach |
|------|--------|----------|
| G204 | 2     | nolint (acceptance test helper) |
| G703 | 3     | nolint + comment (paths under controlled export volume) or add path validation |
| G704 | 3     | nolint (proxy/registry, config-driven URLs) |
| G117 | 8     | nolint on each field (intentional auth/config structs) |
| G705 | 3     | nolint (stderr debug, not HTML) |

**Alternative:** In `.golangci.yml` under `linters.settings.gosec` you already have `excludes: ["G304", "G301"]`. You could add more rules (e.g. `G703`, `G704`, `G705`, `G117`, `G204`) to disable them globally. Prefer inline nolint with a one-line reason so future readers know the finding was considered.

**Makefile lint deprecation:** The warning `Flag --skip-files has been deprecated` comes from golangci-lint. Update the `lint` target to use the new way to skip files (e.g. path exclusions in `.golangci.yml` under `run.exclusions.paths` instead of `--skip-files`).
