# Staging Configuration Override

This guide explains how to override staging workload configurations at build trigger-time using the Epinio API.

## Overview

Epinio allows you to override staging workload configurations when triggering a build, without modifying the base configuration stored in ConfigMaps. This enables per-build customization for specific use cases such as:

- Different resource requirements for specific builds
- Testing with different node selectors
- Temporary configuration changes without modifying base settings
- Build-specific optimizations

## Base Configuration

Staging workload configurations are typically defined in ConfigMaps with the label `app.kubernetes.io/component=epinio-staging`. These provide the default configuration for all staging jobs. The base configuration includes:

- Resource requests and limits
- Node selectors
- Tolerations
- Affinity rules
- TTL (time-to-live) for completed jobs
- Storage configuration

## Override Configuration

You can override any of these settings by including a `stagingConfig` field in your `StageRequest` when calling the staging API endpoint.

### API Endpoint

```
POST /api/v1/namespaces/{namespace}/applications/{app}/stage
```

### Request Body

```json
{
  "app": {
    "name": "myapp",
    "namespace": "workspace"
  },
  "blobuid": "abc123...",
  "stagingConfig": {
    "resources": {
      "requests": {
        "cpu": "500m",
        "memory": "2Gi"
      },
      "limits": {
        "cpu": "1000m",
        "memory": "4Gi"
      }
    },
    "nodeSelector": {
      "kubernetes.io/os": "linux"
    },
    "ttlSecondsAfterFinished": 600
  }
}
```

## Configuration Fields

### Resources

Override CPU and memory requests and limits:

```json
{
  "stagingConfig": {
    "resources": {
      "requests": {
        "cpu": "500m",
        "memory": "2Gi"
      },
      "limits": {
        "cpu": "1000m",
        "memory": "4Gi"
      }
    }
  }
}
```

- **Partial overrides**: You can override only `requests` or only `limits`; the other will be preserved from base configuration
- **Format**: Use Kubernetes quantity format (e.g., "500m", "1Gi", "1000")
- **Validation**: Invalid quantities are logged as warnings and base values are used

### NodeSelector

Completely replace the node selector:

```json
{
  "stagingConfig": {
    "nodeSelector": {
      "kubernetes.io/os": "linux",
      "node-type": "gpu"
    }
  }
}
```

- **Note**: NodeSelector is completely replaced (not merged) when provided
- **Empty map**: An empty map is treated as "not provided" and base values are preserved

### Tolerations

Override pod tolerations:

```json
{
  "stagingConfig": {
    "tolerations": [
      {
        "key": "special-node",
        "operator": "Equal",
        "value": "true",
        "effect": "NoSchedule"
      }
    ]
  }
}
```

- **Format**: Standard Kubernetes Toleration objects
- **Empty array**: An empty array is treated as "not provided" and base values are preserved

### Affinity

Override pod affinity rules:

```json
{
  "stagingConfig": {
    "affinity": {
      "nodeAffinity": {
        "requiredDuringSchedulingIgnoredDuringExecution": {
          "nodeSelectorTerms": [
            {
              "matchExpressions": [
                {
                  "key": "kubernetes.io/arch",
                  "operator": "In",
                  "values": ["amd64"]
                }
              ]
            }
          ]
        }
      }
    }
  }
}
```

- **Format**: Standard Kubernetes Affinity object
- **Validation**: Invalid affinity configurations are logged as warnings and base values are used

### TTLSecondsAfterFinished

Override the time-to-live for completed jobs:

```json
{
  "stagingConfig": {
    "ttlSecondsAfterFinished": 600
  }
}
```

- **Values**: Must be >= 0
- **0**: Means no TTL (job will not be automatically deleted)
- **Validation**: Negative values are rejected and base configuration is used

### ServiceAccountName

Override the service account:

```json
{
  "stagingConfig": {
    "serviceAccountName": "custom-sa"
  }
}
```

- **Validation**: Must follow DNS-1123 subdomain format (lowercase alphanumeric, hyphens, dots, max 253 chars)
- **Invalid names**: Are rejected with a warning and base configuration is used

### Storage Configuration

Override storage settings for cache and source blobs:

```json
{
  "stagingConfig": {
    "storage": {
      "cache": {
        "size": "2Gi",
        "storageClassName": "fast-ssd",
        "volumeMode": "Filesystem",
        "accessModes": ["ReadWriteOnce"],
        "emptyDir": false
      },
      "sourceBlobs": {
        "size": "1Gi",
        "storageClassName": "standard",
        "volumeMode": "Filesystem",
        "accessModes": ["ReadWriteOnce", "ReadOnlyMany"],
        "emptyDir": false
      }
    }
  }
}
```

#### Storage Fields

- **size**: Storage size (e.g., "1Gi", "500Mi")
- **storageClassName**: Kubernetes storage class name
- **volumeMode**: Either "Filesystem" or "Block"
- **accessModes**: Array of valid access modes:
  - `ReadWriteOnce`
  - `ReadOnlyMany`
  - `ReadWriteMany`
- **emptyDir**: Boolean - if true, use emptyDir volume instead of PVC

**Validation**:
- Invalid `VolumeMode` values are rejected (must be "Filesystem" or "Block")
- Invalid `AccessModes` are skipped; valid ones are used
- Empty arrays are treated as "not provided" and base values are preserved

## Merge Semantics

The override configuration is merged with the base configuration using the following rules:

1. **Scalar fields** (ServiceAccountName, TTLSecondsAfterFinished): Override completely replaces base if provided
2. **Maps** (NodeSelector): Override completely replaces base (not merged)
3. **Arrays** (Tolerations, AccessModes): Override replaces base if provided and non-empty
4. **Structs** (Resources, Storage, Affinity): Fields are merged individually; override values take precedence
5. **Empty values**: Empty arrays/maps/strings are treated as "not provided" and base values are preserved

## Examples

### Example 1: Override Resources Only

```json
{
  "app": {"name": "myapp", "namespace": "workspace"},
  "blobuid": "abc123",
  "stagingConfig": {
    "resources": {
      "requests": {"cpu": "500m", "memory": "2Gi"}
    }
  }
}
```

This will:
- Use override for CPU/memory requests
- Preserve base configuration for limits
- Preserve all other base settings

### Example 2: Override Multiple Fields

```json
{
  "app": {"name": "myapp", "namespace": "workspace"},
  "blobuid": "abc123",
  "stagingConfig": {
    "resources": {
      "requests": {"cpu": "500m", "memory": "2Gi"},
      "limits": {"cpu": "1000m", "memory": "4Gi"}
    },
    "nodeSelector": {"kubernetes.io/os": "linux"},
    "ttlSecondsAfterFinished": 600
  }
}
```

### Example 3: Using cURL

```bash
curl -X POST "https://your-epinio-server/api/v1/namespaces/workspace/applications/myapp/stage" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "app": {"name": "myapp", "namespace": "workspace"},
    "blobuid": "YOUR_BLOB_UID",
    "stagingConfig": {
      "resources": {
        "requests": {"cpu": "500m", "memory": "2Gi"}
      }
    }
  }'
```

## Error Handling

When invalid values are provided:

1. **Warnings are logged** in the Epinio server logs
2. **Base configuration is used** for the invalid field
3. **Request still succeeds** - the staging job is created with valid overrides and base values for invalid ones

### Common Validation Errors

- **Invalid ServiceAccountName**: Must be DNS-1123 compliant
- **Invalid resource quantities**: Must be valid Kubernetes quantity format
- **Invalid AccessModes**: Must be one of ReadWriteOnce, ReadOnlyMany, ReadWriteMany
- **Invalid VolumeMode**: Must be "Filesystem" or "Block"
- **Negative TTL**: Must be >= 0

## Verifying Overrides

After staging, you can verify the configuration was applied:

```bash
# List staging jobs
kubectl get jobs -n epinio -l app.kubernetes.io/name=myapp

# Check resources
kubectl get job <job-name> -n epinio -o jsonpath='{.spec.template.spec.containers[0].resources}'

# Check nodeSelector
kubectl get job <job-name> -n epinio -o jsonpath='{.spec.template.spec.nodeSelector}'

# Check TTL
kubectl get job <job-name> -n epinio -o jsonpath='{.spec.ttlSecondsAfterFinished}'
```

## Server Logs

Check Epinio server logs for merge warnings and validation messages:

```bash
kubectl logs -n epinio deployment/epinio-server | grep "staging config override"
```

## Backward Compatibility

This feature is **fully backward compatible**. Existing API calls without `stagingConfig` continue to work unchanged, using only the base ConfigMap configuration.

## See Also

- [Staging Configuration Internals](../explanations/configuration-management-internals.md)
- [API Reference](../references/api/swagger.json)

