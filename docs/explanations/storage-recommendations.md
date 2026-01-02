# Storage recommendations by scale of users/apps

## Overview

Storage configuration and requirements are critical for maintaining Epinio. Users that underprovision storage space could be blocked fairly quickly as users continue to onboard to a particular instance. This document provides guidance on storage requirements and recommendations based on the scale of your Epinio deployment.

## Storage Components

Epinio uses several storage components that need to be considered when planning your deployment:

### 1. Application Source Blobs (S3 Storage)

**Purpose**: Stores uploaded application source code (tarballs, archives) before staging.

**Storage Type**: S3-compatible object storage (internal MinIO or external S3)

**Calculation**:
- Each application upload creates one blob in S3
- Storage = `Number of apps × Average source blob size × Retention factor`
- Typical source blob sizes: 10-100 MB (small apps), 100-500 MB (medium apps), 500 MB-2 GB (large apps)
- Retention factor: Consider how many versions/upload attempts you want to retain (default: all)

**Example**:
- 50 apps with average 50 MB source blobs = ~2.5 GB minimum
- With 3 versions per app retained = ~7.5 GB

**Configuration Impact**:
- **Internal S3 (MinIO)**: Storage is provisioned within the cluster via PVC
- **External S3**: Storage is managed externally (AWS S3, MinIO, etc.) - no cluster storage needed, but requires external capacity planning

### 2. Application Build Cache (PVC)

**Purpose**: Stores build cache to speed up subsequent builds of the same application.

**Storage Type**: Kubernetes PersistentVolumeClaim (PVC) or EmptyDir

**Default Size**: 1 GiB per application (if not specified)

**Calculation**:
- If using PVCs: `Number of apps × Cache size per app`
- Default: `N apps × 1 GiB = N GiB`
- Cache size depends on buildpack layers and application dependencies
- Typical cache sizes: 500 MB - 2 GB per app

**Configuration Options**:
- **Persistent (PVC)**: Cache persists across builds, faster rebuilds, requires storage
- **EmptyDir**: Ephemeral, no storage provisioning needed, but slower rebuilds

**Example**:
- 50 apps with 1 GiB cache each = 50 GiB total

### 3. Application Source Blobs (PVC)

**Purpose**: Stores application source code during staging process.

**Storage Type**: Kubernetes PersistentVolumeClaim (PVC) or EmptyDir

**Default Size**: 1 GiB per application (if not specified)

**Calculation**:
- If using PVCs: `Number of apps × Source blob size per app`
- Default: `N apps × 1 GiB = N GiB`
- Should accommodate largest application source archive

**Configuration Options**:
- **Persistent (PVC)**: Source persists, allows reuse, requires storage
- **EmptyDir**: Ephemeral, downloaded fresh each time, no storage provisioning needed

**Example**:
- 50 apps with 1 GiB source storage each = 50 GiB total

### 4. Container Registry

**Purpose**: Stores built application container images.

**Storage Type**: Container registry storage (internal or external)

**Calculation**:
- Storage = `Number of apps × Number of versions × Average image size`
- Image size = Base image layers + Application layers + Buildpack layers
- Typical image sizes: 100-300 MB (small apps), 300-800 MB (medium apps), 800 MB-2 GB (large apps)
- Each new stage creates a new image version

**Example**:
- 50 apps with average 500 MB images, 3 versions each = ~75 GB

**Configuration Impact**:
- **Internal Registry**: Storage is provisioned within the cluster
- **External Registry**: Storage is managed externally (Docker Hub, AWS ECR, etc.) - no cluster storage needed

## Storage Calculation Formulas

### Total Cluster Storage (Internal Components Only)

If using internal S3 and internal registry:

```
Total Storage = S3 Storage + (Cache PVCs + Source Blobs PVCs) + Registry Storage
```

Where:
- **S3 Storage** = `N apps × Avg source blob size × Retention factor`
- **Cache PVCs** = `N apps × Cache size` (if using PVCs, else 0)
- **Source Blobs PVCs** = `N apps × Source blob size` (if using PVCs, else 0)
- **Registry Storage** = `N apps × Avg image size × Versions per app`

### Simplified Formula

For quick estimation with default configurations:

```
Total Storage (GB) ≈ N apps × (Source blob size + Cache size + Source PVC size + Image size × Versions)
```

With typical defaults (1 GiB cache, 1 GiB source PVC, 50 MB source blob, 500 MB image, 3 versions):
```
Total Storage (GB) ≈ N apps × (0.05 + 1 + 1 + 0.5 × 3) = N apps × 3.55 GB
```

## Recommendations by Scale

### Small Scale (1-10 apps)

**Storage Requirements**:
- **S3 Storage**: 1-5 GB (depending on source blob sizes)
- **PVC Storage** (if using persistent): 20-40 GB (2-4 GB per app)
- **Registry Storage**: 5-30 GB (depending on image sizes and versions)
- **Total**: ~30-75 GB

**Recommendations**:
- Default 1 GiB PVC sizes are typically sufficient
- Can use EmptyDir for cache/source if storage is constrained
- Internal S3 and registry are fine for this scale

### Medium Scale (10-50 apps)

**Storage Requirements**:
- **S3 Storage**: 5-25 GB
- **PVC Storage** (if using persistent): 100-200 GB
- **Registry Storage**: 30-150 GB
- **Total**: ~150-400 GB

**Recommendations**:
- Monitor PVC usage and adjust sizes based on actual application sizes
- Consider external S3 if cluster storage is limited
- Plan for registry storage growth as apps are updated
- Consider using storage classes with dynamic provisioning

### Large Scale (50-200 apps)

**Storage Requirements**:
- **S3 Storage**: 25-100 GB
- **PVC Storage** (if using persistent): 200-800 GB
- **Registry Storage**: 150-600 GB
- **Total**: ~400 GB - 1.5 TB

**Recommendations**:
- **Strongly consider external S3** to reduce cluster storage pressure
- Use external container registry for better scalability
- Implement storage quotas and monitoring
- Use appropriate storage classes (e.g., SSD for cache, standard for source)
- Plan for regular cleanup of old images and source blobs

### Enterprise Scale (200+ apps)

**Storage Requirements**:
- **S3 Storage**: 100+ GB
- **PVC Storage** (if using persistent): 800+ GB
- **Registry Storage**: 600+ GB
- **Total**: 1.5+ TB

**Recommendations**:
- **Use external S3** (AWS S3, MinIO, etc.)
- **Use external container registry** (AWS ECR, Harbor, etc.)
- Implement automated cleanup policies
- Use storage classes optimized for workload (high IOPS for cache)
- Monitor and alert on storage usage
- Consider storage tiering strategies

## Configuration Options and Their Impact

### Storage Class Configuration

**Impact**: Different storage classes have different performance characteristics and costs.

- **Standard/HDD**: Lower cost, suitable for source blobs and S3
- **SSD/Fast**: Higher cost, better for cache volumes that benefit from IOPS
- **External Storage Classes**: May have different provisioning models (e.g., cloud storage with different tiers)

**Recommendation**: Use faster storage classes for cache volumes if budget allows, as they significantly improve build times.

### EmptyDir vs Persistent Volumes

**EmptyDir (Ephemeral)**:
- **Pros**: No storage provisioning needed, no PVC management
- **Cons**: Slower builds (no cache reuse), data lost on pod restart
- **Use Case**: Development environments, storage-constrained deployments

**Persistent Volumes (PVC)**:
- **Pros**: Faster builds (cache reuse), data persistence
- **Cons**: Requires storage provisioning, PVC management overhead
- **Use Case**: Production environments, frequent rebuilds

### External S3 Configuration

**Impact**: Moves source blob storage outside the cluster.

- **Pros**: Reduces cluster storage requirements, better scalability, can leverage cloud storage features
- **Cons**: Requires external S3 setup and credentials, network dependency
- **Storage Impact**: Eliminates S3 storage from cluster storage calculations

**When to Use**:
- Medium to large scale deployments
- Limited cluster storage capacity
- Need for S3-specific features (lifecycle policies, cross-region replication, etc.)

### External Container Registry

**Impact**: Moves container image storage outside the cluster.

- **Pros**: Reduces cluster storage requirements, better scalability, can leverage registry features
- **Cons**: Requires external registry setup, network dependency
- **Storage Impact**: Eliminates registry storage from cluster storage calculations

**When to Use**:
- Medium to large scale deployments
- Limited cluster storage capacity
- Need for registry-specific features (vulnerability scanning, image signing, etc.)

## Application Size Considerations

**Critical Note**: Storage requirements are highly dependent on application size. The formulas above use averages, but actual requirements can vary significantly.

**Factors Affecting Storage**:
- **Source Code Size**: Larger applications require more S3 and source PVC storage
- **Dependencies**: Applications with many dependencies create larger build caches and images
- **Buildpack Layers**: Different buildpacks add different amounts to image size
- **Multi-stage Builds**: Applications with multiple build stages may create larger intermediate artifacts

**Recommendation**: Monitor actual usage and adjust storage sizes based on your specific application profiles. Start with defaults and scale based on observed usage patterns.

## Monitoring and Maintenance

### Key Metrics to Monitor

1. **PVC Usage**: Monitor usage of cache and source blob PVCs
2. **S3 Bucket Size**: Track S3 bucket growth over time
3. **Registry Storage**: Monitor container registry storage usage
4. **Storage Class Performance**: Track IOPS and latency for cache volumes

### Cleanup Strategies

1. **Old Source Blobs**: Implement lifecycle policies to delete old S3 objects
2. **Old Images**: Configure registry garbage collection for unused images
3. **Unused PVCs**: Clean up PVCs for deleted applications
4. **Build Cache**: Periodically clear build caches for applications that haven't been rebuilt recently

## Example Scenarios

### Scenario 1: Small Development Environment
- **10 apps**, average 30 MB source, 200 MB images, 2 versions
- **Configuration**: Internal S3, internal registry, EmptyDir for cache/source
- **Storage Needed**: ~3 GB (S3) + ~4 GB (registry) = **~7 GB total**

### Scenario 2: Medium Production Environment
- **30 apps**, average 100 MB source, 500 MB images, 5 versions
- **Configuration**: Internal S3, internal registry, PVCs (1 GiB each)
- **Storage Needed**: ~3 GB (S3) + ~60 GB (PVCs) + ~75 GB (registry) = **~140 GB total**

### Scenario 3: Large Production Environment
- **100 apps**, average 150 MB source, 800 MB images, 10 versions
- **Configuration**: External S3, external registry, PVCs (2 GiB cache, 1 GiB source)
- **Storage Needed**: ~300 GB (PVCs) + External S3/Registry = **~300 GB cluster storage** (S3/registry external)

## Summary

Storage requirements for Epinio scale with:
- Number of applications
- Application and image sizes
- Number of versions retained
- Storage configuration choices (PVC vs EmptyDir, internal vs external)

**Key Takeaway**: Our storage hinges largely on application size, so ensure you make note of this in your planning. Monitor actual usage and adjust accordingly. For larger deployments, consider external S3 and registry to reduce cluster storage pressure.

## Further Reading

### Epinio-Specific Resources

- **[Epinio Meets s3gw](https://www.suse.com/c/rancher_blog/epino-meets-s3gw/)** - Blog post discussing integrating Epinio with s3gw, a lightweight S3-compatible service, highlighting storage configurations and considerations.

- **[Customizing and Securing Your Epinio Installation](https://www.suse.com/c/rancher_blog/customizing-and-securing-your-epinio-installation/)** - Article exploring various customization options for Epinio, including asset storage configurations and guidance on integrating external object storage and registries.

- **[How to Setup External S3 Storage](https://docs.epinio.io/1.6.1/howtos/setup_external_s3)** - Epinio documentation guide on configuring external S3-compatible storage for more flexible and scalable storage solutions.

### General Storage Planning and Best Practices

- **[Eight Things to Consider Before Moving Your Storage Backup to the Cloud](https://www.eweek.com/storage/eight-things-to-consider-before-moving-your-storage-backup-to-the-cloud/)** - Key factors to evaluate when transitioning storage backups to cloud environments, including data security and mobility considerations.

- **[Data Resilience: Eon's Approach & Google Cloud Best Practices](https://cloud.google.com/blog/products/storage-data-transfer/data-resilience-eons-approach--google-cloud-best-practices)** - Best practices for data protection in cloud environments, emphasizing versioning, retention policies, and designing for granular recovery.

- **[Top 10 Tips and Tricks for Storage in the Cloud](https://bluexp.netapp.com/hubfs/Paid-Tips-and-Tricks-to-Storage-in-the-Cloud-eBook.pdf)** - eBook offering practical advice for optimizing cloud storage, covering topics like cost management, data security, and performance optimization.

### Kubernetes Storage Resources

- **[Kubernetes Storage Documentation](https://kubernetes.io/docs/concepts/storage/)** - Official Kubernetes documentation on storage concepts, including PersistentVolumes, StorageClasses, and volume types.

- **[Kubernetes Storage Best Practices](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#best-practices)** - Kubernetes best practices for managing persistent storage in production environments.

