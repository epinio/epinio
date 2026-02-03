# Version Parity Validation

## Overview

Version parity validation ensures that the Epinio binary release version matches the Helm chart version. This prevents issues where users might try to install a Helm chart that references CLI binaries that don't exist.

## Background

Issue [#2774](https://github.com/epinio/epinio/issues/2774) identified a problem where the Helm chart version (v1.11.1) did not correspond with the available Epinio release binaries. Users attempting to download the CLI for that version would encounter 404 errors because the binaries were never released.

## How It Works

The validation system includes:

1. **Validation Script** (`scripts/validate-version-parity.sh`): A bash script that compares the Epinio version with the Helm chart version
2. **GitHub Actions Workflow** (`.github/workflows/validate-version-parity.yml`): Automated validation on PRs and scheduled checks
3. **Release Gate**: Validation step in the release workflow to catch mismatches before release
4. **Makefile Targets**: Easy local validation commands

## Usage

### Local Validation

Check version parity locally:

```bash
make validate-version-parity
```

Strict validation (fails on mismatch):

```bash
make validate-version-parity-strict
```

Direct script usage:

```bash
# Check mode (default)
./scripts/validate-version-parity.sh check

# Warning mode (warns but doesn't fail)
./scripts/validate-version-parity.sh warn

# Strict mode (fails on mismatch)
./scripts/validate-version-parity.sh strict

# With custom Chart.yaml path
./scripts/validate-version-parity.sh check helm-charts/chart/epinio/Chart.yaml

# Override version
EPINIO_VERSION=v1.13.0 ./scripts/validate-version-parity.sh check
```

### CI/CD Integration

The validation runs automatically:

- **On Pull Requests**: When version-related files are modified
- **On Main Branch**: When version files are updated
- **Weekly**: Every Monday to catch any version drift
- **On Release**: Before creating a new release (strict mode)
- **Manual Trigger**: Can be triggered manually via GitHub Actions UI

### Validation Modes

| Mode | Behavior | Use Case |
|------|----------|----------|
| `check` | Reports version mismatch but doesn't fail | Default mode for information |
| `warn` | Shows warnings and proceeds | Local testing, manual checks |
| `strict` | Fails on version mismatch | PR validation, release workflow, pre-release gates |

## Release Process

### Current Behavior

During a release, the validation runs in **strict mode**:
- ✅ Reports if versions match and allows release to proceed
- ❌ **Blocks release** if versions mismatch (prevents issue #2774)
- 📝 Creates detailed error messages for quick resolution

On pull requests, the validation also runs in **strict mode**:
- ✅ Validates changes to `internal/version/version.go` against helm chart
- ❌ Fails the PR check if versions don't match
- 💬 Posts a detailed comment on the PR with fix instructions

## Troubleshooting

### Version Mismatch Detected

If validation reports a version mismatch:

1. **Check the Epinio version**:
   ```bash
   git describe --tags --abbrev=0
   ```

2. **Check the Helm chart version**:
   ```bash
   cat helm-charts/chart/epinio/Chart.yaml | grep 'appVersion'
   ```

3. **Update the helm-charts submodule**:
   ```bash
   git submodule update --init --recursive helm-charts
   cd helm-charts
   git checkout main
   git pull origin main
   cd ..
   git add helm-charts
   git commit -m "Update helm-charts submodule"
   ```

4. **Coordinate with helm-charts repository**: Ensure the helm-charts repository has the correct version before releasing

### Helm-charts Submodule Not Initialized

The validation script can work without the submodule:
- It will automatically attempt to initialize the submodule
- If that fails, it fetches the Chart.yaml from the remote repository
- This ensures validation works in CI environments

### Unknown Versions

If versions cannot be determined:
- In `check` or `warn` mode: Validation is skipped with a warning
- In `strict` mode: Validation fails to prevent releasing with unknown versions

## Best Practices

1. **Before Creating a Release Tag**:
   ```bash
   make validate-version-parity-strict
   ```

2. **After Updating helm-charts Submodule**:
   ```bash
   make validate-version-parity
   ```

3. **In CI/CD Pipelines**: Use `warn` mode to alert but not block
4. **For Release Gates**: Use `strict` mode to prevent version mismatches

## Architecture

### Version Sources

- **Epinio Version** (priority order):
  1. `EPINIO_VERSION` environment variable (explicit override)
  2. `internal/version/version.go` → `ChartVersion` variable (for PR validation)
  3. `GITHUB_REF_NAME` in GitHub Actions (for release tags)
  4. `git describe --tags` (fallback)

- **Helm Chart Version**:
  - From `helm-charts/chart/epinio/Chart.yaml` (local)
  - From GitHub raw content (remote fallback)
  - Uses `appVersion` field (matches Epinio release version)

### PR Validation Flow

When a PR modifies `internal/version/version.go`:

1. Script reads `ChartVersion` from the modified `version.go`
2. Compares with `appVersion` in helm-charts
3. If mismatch: PR check **fails** and detailed comment is posted
4. If match: PR check passes

This ensures developers update both files together.

### Integration Points

```
┌─────────────────────┐
│  Release Workflow   │
│                     │
│  1. Create Tag      │
│  2. Validate        │◄──── Version Parity Check
│  3. Build Binaries  │
│  4. Release         │
│  5. Notify Helm     │
└─────────────────────┘
```

## Contributing

When adding new version-related features:

1. Update the validation script if version sources change
2. Test all validation modes
3. Update this documentation
4. Consider impact on CI/CD pipelines

## Related Issues

- [#2774](https://github.com/epinio/epinio/issues/2774) - Original issue that motivated this validation

## Questions or Issues

If you encounter problems with version validation:

1. Check the [Troubleshooting](#troubleshooting) section
2. Review GitHub Actions logs
3. Run local validation with verbose output
4. Open an issue with details about the mismatch
