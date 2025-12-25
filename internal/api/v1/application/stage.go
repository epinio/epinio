// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
    "context"
    "encoding/json"
    "fmt"
    "path/filepath"
    "reflect"
    "strconv"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/go-logr/logr"
    "github.com/pkg/errors"
    "github.com/spf13/viper"
    "gopkg.in/yaml.v3"
    batchv1 "k8s.io/api/batch/v1"
    "k8s.io/utils/ptr"

    corev1 "k8s.io/api/core/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/labels"
    "k8s.io/apimachinery/pkg/util/validation"

    "github.com/epinio/epinio/helpers/cahash"
    "github.com/epinio/epinio/helpers/kubernetes"
    "github.com/epinio/epinio/helpers/randstr"
    "github.com/epinio/epinio/internal/api/v1/response"
    "github.com/epinio/epinio/internal/application"
    "github.com/epinio/epinio/internal/cli/server/requestctx"
    "github.com/epinio/epinio/internal/duration"
    "github.com/epinio/epinio/internal/helmchart"
    "github.com/epinio/epinio/internal/names"
    "github.com/epinio/epinio/internal/registry"
    "github.com/epinio/epinio/internal/s3manager"
    apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
    "github.com/epinio/epinio/pkg/api/core/v1/models"
    "golang.org/x/exp/slices"
)

type stageParam struct {
    models.AppRef
    BlobUID             string
    BuilderImage        string
    DownloadImage       string
    UnpackImage         string
    Environment         models.EnvVariableList
    Owner               metav1.OwnerReference
    RegistryURL         string
    S3ConnectionDetails s3manager.ConnectionDetails
    Stage               models.StageRef
    Username            string
    PreviousStageID     string
    RegistryCASecret    string
    RegistryCAHash      string
    UserID              int64
    GroupID             int64
    Scripts             string
    HelmValues          HelmValuesMap  // Helm Values configuring the staging workload
}

type HelmValuesMap struct {
    ServiceAccountName      string                         `json:"serviceAccountName"`
    NodeSelector            map[string]string              `json:"nodeSelector,omitempty"`
    Tolerations             []corev1.Toleration            `json:"tolerations,omitempty"`
    Affinity                corev1.Affinity                `json:"affinity,omitempty"`
    Resources               corev1.ResourceRequirements    `json:"resources,omitempty"`
    TTLSecondsAfterFinished int32                          `json:"ttlSecondsAfterFinished,omitempty"`
    Storage                 struct {
        SourceBlobs              StagingStorageValues      `json:"sourceBlobs,omitempty"`
        Cache                    StagingStorageValues      `json:"cache,omitempty"`
    } `json:"storage,omitempty"`
}

type StagingStorageValues struct {
    Size                string                               `json:"size,omitempty"`
    StorageClassName    string                               `json:"storageClassName,omitempty"`
    VolumeMode          corev1.PersistentVolumeMode          `json:"volumeMode,omitempty"`
    AccessModes         []corev1.PersistentVolumeAccessMode  `json:"accessModes,omitempty"`
    EmptyDir            bool                                 `json:"emptyDir,omitempty"`
}

// ImageURL returns the URL of the container image to be, using the
// ImageID. The ImageURL is later used in app.yml and to send in the
// stage response.
func (app *stageParam) ImageURL(registryURL string) string {
    return fmt.Sprintf("%s/%s-%s:%s", registryURL, app.Namespace, app.Name, app.Stage.ID)
}

// ensurePVC creates a PVC for the application if one doesn't already exist.
// This PVC is used to store the application source blobs (as they are uploaded
// on the "upload" endpoint). It is also mounted in the staging pod, as the
// "source" workspace.
// The same PVC stores the application's build cache (on a separate directory).
func ensurePVC(ctx context.Context, cluster *kubernetes.Cluster, config StagingStorageValues, pvcName string) error {
    _, err := cluster.Kubectl.CoreV1().PersistentVolumeClaims(helmchart.Namespace()).
        Get(ctx, pvcName, metav1.GetOptions{})
    if err != nil && !apierrors.IsNotFound(err) { // Unknown error, irrelevant to non-existence
        return err
    }
    if err == nil { // pvc already exists
        return nil
    }

    // Insert a default of last resort. See also note below.
    if config.Size == "" {
        config.Size = "1Gi"
    }

    if len(config.AccessModes) == 0 {
        config.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
    }

    if config.VolumeMode == "" {
        config.VolumeMode = corev1.PersistentVolumeFilesystem
    }


    pvcObject := &corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name:      pvcName,
            Namespace: helmchart.Namespace(),
        },
        Spec: corev1.PersistentVolumeClaimSpec{
            AccessModes: config.AccessModes,
            VolumeMode: &config.VolumeMode,
            Resources: corev1.VolumeResourceRequirements{
                Requests: map[corev1.ResourceName]resource.Quantity{
                    corev1.ResourceStorage: resource.MustParse(config.Size),
                },
            },
        },
    }

    if config.StorageClassName != "" {
        pvcObject.Spec.StorageClassName = &config.StorageClassName
    }


    // From here on, only if the PVC is missing
    _, err = cluster.Kubectl.CoreV1().PersistentVolumeClaims(helmchart.Namespace()).
        Create(ctx, pvcObject, metav1.CreateOptions{})

    return err
}

// getTTLPointer returns a pointer to the TTL value if it's greater than 0,
// otherwise returns nil. This prevents Kubernetes from interpreting 0 as "delete immediately".
func getTTLPointer(ttl int32) *int32 {
    if ttl > 0 {
        return &ttl
    }
    return nil
}

// getAffinityPointer returns a pointer to the Affinity if it's not a zero value,
// otherwise returns nil to avoid unnecessary zero-value structs.
func getAffinityPointer(affinity corev1.Affinity) *corev1.Affinity {
    if reflect.DeepEqual(affinity, corev1.Affinity{}) {
        return nil
    }
    return &affinity
}

// mergeStagingConfig merges base staging configuration with trigger-time overrides.
// The override takes precedence over the base configuration for any provided fields.
// Merge semantics:
//   - Scalar fields (ServiceAccountName, TTLSecondsAfterFinished): Override replaces base if provided
//   - Maps (NodeSelector): Override completely replaces base (not merged)
//   - Arrays (Tolerations, AccessModes): Override replaces base if provided and non-empty
//   - Structs (Resources, Storage, Affinity): Fields are merged, override values take precedence
//   - Empty arrays/maps in override are treated as "not provided" and base values are preserved
func mergeStagingConfig(base HelmValuesMap, override *models.StagingConfig, log logr.Logger) HelmValuesMap {
    if override == nil {
        return base
    }

    // Deep copy to prevent mutating the base configuration
    // Maps and slices need to be copied to avoid shared references
    result := base
    
    // Deep copy NodeSelector map
    if base.NodeSelector != nil {
        result.NodeSelector = make(map[string]string, len(base.NodeSelector))
        for k, v := range base.NodeSelector {
            result.NodeSelector[k] = v
        }
    }
    
    // Deep copy Tolerations slice
    if base.Tolerations != nil {
        result.Tolerations = make([]corev1.Toleration, len(base.Tolerations))
        copy(result.Tolerations, base.Tolerations)
    }
    
    // Deep copy Resources maps
    if base.Resources.Requests != nil {
        result.Resources.Requests = make(map[corev1.ResourceName]resource.Quantity, len(base.Resources.Requests))
        for k, v := range base.Resources.Requests {
            result.Resources.Requests[k] = v
        }
    }
    if base.Resources.Limits != nil {
        result.Resources.Limits = make(map[corev1.ResourceName]resource.Quantity, len(base.Resources.Limits))
        for k, v := range base.Resources.Limits {
            result.Resources.Limits[k] = v
        }
    }
    
    // Deep copy Storage AccessModes slices
    if base.Storage.SourceBlobs.AccessModes != nil {
        result.Storage.SourceBlobs.AccessModes = make([]corev1.PersistentVolumeAccessMode, len(base.Storage.SourceBlobs.AccessModes))
        copy(result.Storage.SourceBlobs.AccessModes, base.Storage.SourceBlobs.AccessModes)
    }
    if base.Storage.Cache.AccessModes != nil {
        result.Storage.Cache.AccessModes = make([]corev1.PersistentVolumeAccessMode, len(base.Storage.Cache.AccessModes))
        copy(result.Storage.Cache.AccessModes, base.Storage.Cache.AccessModes)
    }

    // Merge ServiceAccountName
    // Note: ServiceAccountName must follow DNS-1123 subdomain format (lowercase alphanumeric,
    // hyphens, dots, max 253 chars, must start and end with alphanumeric).
    if override.ServiceAccountName != "" {
        errorMsgs := validation.IsDNS1123Subdomain(override.ServiceAccountName)
        if len(errorMsgs) > 0 {
            log.Info("staging config override", "warning", fmt.Sprintf("invalid ServiceAccountName '%s': %s, using base configuration", override.ServiceAccountName, strings.Join(errorMsgs, "; ")))
        } else {
            result.ServiceAccountName = override.ServiceAccountName
            log.Info("staging config override", "serviceAccountName", override.ServiceAccountName)
        }
    }

    // Merge NodeSelector
    // Note: NodeSelector is completely replaced (not merged) when provided.
    // Empty map is treated as "not provided" and base values are preserved.
    if override.NodeSelector != nil && len(override.NodeSelector) > 0 {
        // Create a copy to avoid sharing the reference with the override
        result.NodeSelector = make(map[string]string, len(override.NodeSelector))
        for k, v := range override.NodeSelector {
            result.NodeSelector[k] = v
        }
        log.Info("staging config override", "nodeSelector", override.NodeSelector)
    }

    // Merge Tolerations
    // Note: Empty array in override is treated as "not provided" and base values are preserved.
    // To clear tolerations, explicitly set them to an empty array in the base config.
    if override.Tolerations != nil && len(override.Tolerations) > 0 {
        tolerations := make([]corev1.Toleration, 0, len(override.Tolerations))
        var conversionErrors []string
        for i, tol := range override.Tolerations {
            // Convert interface{} to Toleration via JSON marshaling/unmarshaling
            tolBytes, err := json.Marshal(tol)
            if err != nil {
                conversionErrors = append(conversionErrors, fmt.Sprintf("toleration[%d]: marshal failed: %v", i, err))
                continue
            }
            var toleration corev1.Toleration
            if err := json.Unmarshal(tolBytes, &toleration); err != nil {
                conversionErrors = append(conversionErrors, fmt.Sprintf("toleration[%d]: unmarshal failed: %v", i, err))
                continue
            }
            tolerations = append(tolerations, toleration)
        }
        if len(conversionErrors) > 0 {
            log.Info("staging config override", "warning", fmt.Sprintf("failed to convert %d toleration(s): %v", len(conversionErrors), conversionErrors))
        }
        if len(tolerations) > 0 {
            result.Tolerations = tolerations
            log.Info("staging config override", "tolerations", len(tolerations))
        } else if len(conversionErrors) > 0 {
            // All tolerations failed conversion, warn but keep base values
            log.Info("staging config override", "warning", "all tolerations failed conversion, using base configuration")
        }
    }

    // Merge Affinity
    if override.Affinity != nil && len(override.Affinity) > 0 {
        affinityBytes, err := json.Marshal(override.Affinity)
        if err != nil {
            log.Info("staging config override", "warning", fmt.Sprintf("failed to marshal affinity: %v, using base configuration", err))
        } else {
            var affinity corev1.Affinity
            if err := json.Unmarshal(affinityBytes, &affinity); err != nil {
                log.Info("staging config override", "warning", fmt.Sprintf("failed to unmarshal affinity: %v, using base configuration", err))
            } else {
                result.Affinity = affinity
                log.Info("staging config override", "affinity", "provided")
            }
        }
    }

    // Merge Resources
    // Note: Partial overrides are supported - if only Requests or Limits are provided,
    // the other is preserved from base configuration.
    if override.Resources != nil {
        resources := corev1.ResourceRequirements{}
        var parseErrors []string
        var allRequestsFailed, allLimitsFailed bool
        
        // Convert requests
        if override.Resources.Requests != nil && len(override.Resources.Requests) > 0 {
            requests := make(map[corev1.ResourceName]resource.Quantity)
            totalRequests := len(override.Resources.Requests)
            for key, value := range override.Resources.Requests {
                qty, err := resource.ParseQuantity(value)
                if err == nil {
                    requests[corev1.ResourceName(key)] = qty
                } else {
                    parseErrors = append(parseErrors, fmt.Sprintf("request %s=%s: %v", key, value, err))
                }
            }
            allRequestsFailed = len(requests) == 0 && totalRequests > 0
            if len(parseErrors) > 0 {
                successfulCount := totalRequests - len(parseErrors)
                if successfulCount > 0 {
                    log.Info("staging config override", "warning", fmt.Sprintf("failed to parse %d of %d resource request(s): %v (using %d valid request(s))", len(parseErrors), totalRequests, parseErrors, successfulCount))
                } else {
                    log.Info("staging config override", "warning", fmt.Sprintf("failed to parse %d resource request(s): %v", len(parseErrors), parseErrors))
                    log.Info("staging config override", "warning", "all resource requests failed to parse, using base configuration for requests")
                }
                parseErrors = nil // Reset for limits
            }
            if !allRequestsFailed {
                resources.Requests = requests
            } else if result.Resources.Requests != nil {
                // Use already-copied result to avoid sharing reference with base
                resources.Requests = result.Resources.Requests
            }
        } else if result.Resources.Requests != nil {
            // Use already-copied result to avoid sharing reference with base
            resources.Requests = result.Resources.Requests
        }

        // Convert limits
        if override.Resources.Limits != nil && len(override.Resources.Limits) > 0 {
            limits := make(map[corev1.ResourceName]resource.Quantity)
            totalLimits := len(override.Resources.Limits)
            for key, value := range override.Resources.Limits {
                qty, err := resource.ParseQuantity(value)
                if err == nil {
                    limits[corev1.ResourceName(key)] = qty
                } else {
                    parseErrors = append(parseErrors, fmt.Sprintf("limit %s=%s: %v", key, value, err))
                }
            }
            allLimitsFailed = len(limits) == 0 && totalLimits > 0
            if len(parseErrors) > 0 {
                successfulCount := totalLimits - len(parseErrors)
                if successfulCount > 0 {
                    log.Info("staging config override", "warning", fmt.Sprintf("failed to parse %d of %d resource limit(s): %v (using %d valid limit(s))", len(parseErrors), totalLimits, parseErrors, successfulCount))
                } else {
                    log.Info("staging config override", "warning", fmt.Sprintf("failed to parse %d resource limit(s): %v", len(parseErrors), parseErrors))
                    log.Info("staging config override", "warning", "all resource limits failed to parse, using base configuration for limits")
                }
            }
            if !allLimitsFailed {
                resources.Limits = limits
            } else if result.Resources.Limits != nil {
                // Use already-copied result to avoid sharing reference with base
                resources.Limits = result.Resources.Limits
            }
        } else if result.Resources.Limits != nil {
            // Use already-copied result to avoid sharing reference with base
            resources.Limits = result.Resources.Limits
        }

        // Final assignment: use merged resources if valid, otherwise preserve base
        if allRequestsFailed && allLimitsFailed {
            // All override resources failed to parse, explicitly preserve base
            log.Info("staging config override", "warning", "all override resources (requests and limits) failed to parse, using base configuration")
            // result.Resources already contains the copied base resources, no need to reassign
        } else if len(resources.Requests) > 0 || len(resources.Limits) > 0 {
            // Some or all override resources are valid, use merged resources
            result.Resources = resources
            log.Info("staging config override", "resources", "provided")
        } else {
            // Override provided but empty/nil, preserve base resources
            // result.Resources already contains the copied base resources, no need to reassign
        }
        // If both override and base are empty, result.Resources remains zero value (correct)
    }

    // Merge TTLSecondsAfterFinished
    // Note: Values < 0 are invalid. Value 0 means no TTL (handled by getTTLPointer returning nil).
    // Negative values are treated as invalid and logged as a warning.
    if override.TTLSecondsAfterFinished != nil {
        ttl := *override.TTLSecondsAfterFinished
        if ttl < 0 {
            log.Info("staging config override", "warning", fmt.Sprintf("invalid TTLSecondsAfterFinished: %d (must be >= 0), using base configuration", ttl))
        } else {
            result.TTLSecondsAfterFinished = ttl
            log.Info("staging config override", "ttlSecondsAfterFinished", ttl)
        }
    }

    // Merge Storage
    // Note: Storage fields are merged individually. Empty arrays in AccessModes are treated
    // as "not provided" and base values are preserved. To clear AccessModes, set them
    // to an empty array in the base configuration.
    if override.Storage != nil {
        // Merge SourceBlobs storage
        if override.Storage.SourceBlobs != nil {
            // Use the already-copied result storage to avoid mutating base
            sourceBlobs := result.Storage.SourceBlobs
            if override.Storage.SourceBlobs.Size != "" {
                sourceBlobs.Size = override.Storage.SourceBlobs.Size
            }
            if override.Storage.SourceBlobs.StorageClassName != "" {
                sourceBlobs.StorageClassName = override.Storage.SourceBlobs.StorageClassName
            }
            if override.Storage.SourceBlobs.VolumeMode != "" {
                // Validate VolumeMode - Kubernetes accepts "Filesystem" or "Block"
                volumeMode := corev1.PersistentVolumeMode(override.Storage.SourceBlobs.VolumeMode)
                if volumeMode != corev1.PersistentVolumeFilesystem && volumeMode != corev1.PersistentVolumeBlock {
                    log.Info("staging config override", "warning", fmt.Sprintf("invalid VolumeMode: %s (expected Filesystem or Block), using base configuration", override.Storage.SourceBlobs.VolumeMode))
                } else {
                    sourceBlobs.VolumeMode = volumeMode
                }
            }
            // AccessModes: Empty array is treated as "not provided", base values are preserved
            // Valid values: ReadWriteOnce, ReadOnlyMany, ReadWriteMany
            if override.Storage.SourceBlobs.AccessModes != nil && len(override.Storage.SourceBlobs.AccessModes) > 0 {
                accessModes := make([]corev1.PersistentVolumeAccessMode, 0, len(override.Storage.SourceBlobs.AccessModes))
                var invalidModes []string
                validModes := map[string]bool{
                    string(corev1.ReadWriteOnce): true,
                    string(corev1.ReadOnlyMany): true,
                    string(corev1.ReadWriteMany): true,
                }
                for _, mode := range override.Storage.SourceBlobs.AccessModes {
                    modeStr := string(mode)
                    if validModes[modeStr] {
                        accessModes = append(accessModes, corev1.PersistentVolumeAccessMode(mode))
                    } else {
                        invalidModes = append(invalidModes, modeStr)
                    }
                }
                if len(invalidModes) > 0 {
                    log.Info("staging config override", "warning", fmt.Sprintf("invalid AccessModes for sourceBlobs: %v (valid values: ReadWriteOnce, ReadOnlyMany, ReadWriteMany), skipping invalid modes", invalidModes))
                }
                if len(accessModes) > 0 {
                    sourceBlobs.AccessModes = accessModes
                } else {
                    log.Info("staging config override", "warning", "all AccessModes for sourceBlobs were invalid, using base configuration")
                }
            }
            if override.Storage.SourceBlobs.EmptyDir != nil {
                sourceBlobs.EmptyDir = *override.Storage.SourceBlobs.EmptyDir
            }
            result.Storage.SourceBlobs = sourceBlobs
            log.Info("staging config override", "storage.sourceBlobs", "provided")
        }

        // Merge Cache storage
        if override.Storage.Cache != nil {
            // Use the already-copied result storage to avoid mutating base
            cache := result.Storage.Cache
            if override.Storage.Cache.Size != "" {
                cache.Size = override.Storage.Cache.Size
            }
            if override.Storage.Cache.StorageClassName != "" {
                cache.StorageClassName = override.Storage.Cache.StorageClassName
            }
            if override.Storage.Cache.VolumeMode != "" {
                // Validate VolumeMode - Kubernetes accepts "Filesystem" or "Block"
                volumeMode := corev1.PersistentVolumeMode(override.Storage.Cache.VolumeMode)
                if volumeMode != corev1.PersistentVolumeFilesystem && volumeMode != corev1.PersistentVolumeBlock {
                    log.Info("staging config override", "warning", fmt.Sprintf("invalid VolumeMode: %s (expected Filesystem or Block), using base configuration", override.Storage.Cache.VolumeMode))
                } else {
                    cache.VolumeMode = volumeMode
                }
            }
            // AccessModes: Empty array is treated as "not provided", base values are preserved
            // Valid values: ReadWriteOnce, ReadOnlyMany, ReadWriteMany
            if override.Storage.Cache.AccessModes != nil && len(override.Storage.Cache.AccessModes) > 0 {
                accessModes := make([]corev1.PersistentVolumeAccessMode, 0, len(override.Storage.Cache.AccessModes))
                var invalidModes []string
                validModes := map[string]bool{
                    string(corev1.ReadWriteOnce): true,
                    string(corev1.ReadOnlyMany): true,
                    string(corev1.ReadWriteMany): true,
                }
                for _, mode := range override.Storage.Cache.AccessModes {
                    modeStr := string(mode)
                    if validModes[modeStr] {
                        accessModes = append(accessModes, corev1.PersistentVolumeAccessMode(mode))
                    } else {
                        invalidModes = append(invalidModes, modeStr)
                    }
                }
                if len(invalidModes) > 0 {
                    log.Info("staging config override", "warning", fmt.Sprintf("invalid AccessModes for cache: %v (valid values: ReadWriteOnce, ReadOnlyMany, ReadWriteMany), skipping invalid modes", invalidModes))
                }
                if len(accessModes) > 0 {
                    cache.AccessModes = accessModes
                } else {
                    log.Info("staging config override", "warning", "all AccessModes for cache were invalid, using base configuration")
                }
            }
            if override.Storage.Cache.EmptyDir != nil {
                cache.EmptyDir = *override.Storage.Cache.EmptyDir
            }
            result.Storage.Cache = cache
            log.Info("staging config override", "storage.cache", "provided")
        }
    }

    return result
}

// Stage handles the API endpoint /namespaces/:namespace/applications/:app/stage
// It creates a Job resource to stage the app
func Stage(c *gin.Context) apierror.APIErrors {
    ctx := c.Request.Context()
    log := requestctx.Logger(ctx)

    namespace := c.Param("namespace")
    name := c.Param("app")
    username := requestctx.User(ctx).Username

    req := models.StageRequest{}
    if err := c.BindJSON(&req); err != nil {
        return apierror.NewBadRequestError(err.Error()).WithDetails("failed to unmarshal app stage request")
    }
    if name != req.App.Name {
        return apierror.NewBadRequestError("name parameter from URL does not match name param in body")
    }
    if namespace != req.App.Namespace {
        return apierror.NewBadRequestError("namespace parameter from URL does not match namespace param in body")
    }

    cluster, err := kubernetes.GetCluster(ctx)
    if err != nil {
        return apierror.InternalError(err, "failed to get access to a kube client")
    }

    // check application resource
    app, err := application.Get(ctx, cluster, req.App)
    if err != nil {
        if apierrors.IsNotFound(err) {
            return apierror.AppIsNotKnown("cannot stage app, application resource is missing")
        }
        return apierror.InternalError(err, "failed to get the application resource")
    }

    // quickly reject conflict with (still) active staging
    staging, err := application.IsCurrentlyStaging(ctx, cluster, req.App.Namespace, req.App.Name)
    if err != nil {
        return apierror.InternalError(err)
    }
    if staging {
        return apierror.NewBadRequestError("staging job for image ID still running")
    }

    // get builder image from either request, application, or default as final fallback

    builderImage, builderErr := getBuilderImage(req, app)
    if builderErr != nil {
        return builderErr
    }
    if builderImage == "" {
        builderImage = viper.GetString("default-builder-image")
    }

    // Find staging script spec based on the builder image and what images are supported by each spec
    // This also resolves a `base` reference, if present.

    config, err := DetermineStagingScripts(ctx, log, cluster, helmchart.Namespace(), builderImage)
    if err != nil {
        return apierror.InternalError(err, "failed to retrieve staging configuration")
    }

    log.Info("staging app", "scripts", config.Name)
    log.Info("staging app", "builder", builderImage)
    log.Info("staging app", "download", config.DownloadImage)
    log.Info("staging app", "unpack", config.UnpackImage)
    log.Info("staging app", "userid", config.UserID)
    log.Info("staging app", "groupid", config.GroupID)
    log.Info("staging app", "build env", config.Env)
    log.Info("staging app", "Staging Values", config.HelmValues)
    log.Info("staging app", "namespace", namespace, "app", req)

    // Merge base staging configuration with any trigger-time overrides
    helmValues := mergeStagingConfig(config.HelmValues, req.StagingConfig, log)

    s3ConnectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster,
        helmchart.Namespace(), helmchart.S3ConnectionDetailsSecretName)
    if err != nil {
        return apierror.InternalError(err, "failed to fetch the S3 connection details")
    }

    blobUID, blobErr := getBlobUID(ctx, s3ConnectionDetails, req, app)
    if blobErr != nil {
        return blobErr
    }

    // Create uid identifying the staging job to be

    uid, err := randstr.Hex16()
    if err != nil {
        return apierror.InternalError(err, "failed to generate a uid")
    }

    environment, err := application.Environment(ctx, cluster, req.App)
    if err != nil {
        return apierror.InternalError(err, "failed to access application runtime environment")
    }

    owner := metav1.OwnerReference{
        APIVersion: app.GetAPIVersion(),
        Kind:       app.GetKind(),
        Name:       app.GetName(),
        UID:        app.GetUID(),
    }

    // Determine stage id of currently running deployment. Fallback to itself when no such exists.
    // From the view of the new build we are about to create this is the previous id.
    previousID, err := application.StageID(app)
    if err != nil {
        return apierror.InternalError(err, "failed to determine application stage id")
    }
    if previousID == "" {
        previousID = uid
    }

    registryPublicURL, err := getRegistryURL(ctx, cluster)
    if err != nil {
        return apierror.InternalError(err, "getting the Epinio registry public URL")
    }

    registryCertificateSecret := viper.GetString("registry-certificate-secret")
    registryCertificateHash := ""
    if registryCertificateSecret != "" {
        registryCertificateHash, err = getRegistryCertificateHash(ctx, cluster, helmchart.Namespace(), registryCertificateSecret)
        if err != nil {
            return apierror.InternalError(err, "cannot calculate Certificate hash")
        }
    }

    // merge buildpack and general EV, i.e. `config.Env`, and `environment`.
    // ATTENTION: in case of conflict the general, i.e. user-specified!, EV has priority.
    for name, value := range config.Env {
        if _, found := environment[name]; found {
            continue
        }
        environment[name] = value
    }

    params := stageParam{
        AppRef:              req.App,
        BuilderImage:        builderImage,
        DownloadImage:       config.DownloadImage,
        UnpackImage:         config.UnpackImage,
        BlobUID:             blobUID,
        Environment:         environment.List(),
        Owner:               owner,
        RegistryURL:         registryPublicURL,
        S3ConnectionDetails: s3ConnectionDetails,
        Stage:               models.NewStage(uid),
        PreviousStageID:     previousID,
        Username:            username,
        RegistryCAHash:      registryCertificateHash,
        RegistryCASecret:    registryCertificateSecret,
        UserID:              config.UserID,
        GroupID:             config.GroupID,
        Scripts:             config.Name,
        HelmValues:          helmValues,
    }

    if !params.HelmValues.Storage.Cache.EmptyDir {
        err = ensurePVC(ctx, cluster, params.HelmValues.Storage.Cache, req.App.MakeCachePVCName())
        if err != nil {
            return apierror.InternalError(err, "failed to ensure a PersistentVolumeClaim for the application cache")
        }
    }

    if !params.HelmValues.Storage.SourceBlobs.EmptyDir {
        err = ensurePVC(ctx, cluster, params.HelmValues.Storage.SourceBlobs, req.App.MakeSourceBlobsPVCName())
        if err != nil {
            return apierror.InternalError(err, "failed to ensure a PersistentVolumeClaim for the application source blobs")
        }
    }

    job, jobenv := newJobRun(params)

    // Note: The secret is deleted with the job in function `Unstage()`.
    err = cluster.CreateSecret(ctx, helmchart.Namespace(), *jobenv)
    if err != nil {
        return apierror.InternalError(err, fmt.Sprintf("failed to create job env: %#v", jobenv))
    }

    err = cluster.CreateJob(ctx, helmchart.Namespace(), job)
    if err != nil {
        return apierror.InternalError(err, fmt.Sprintf("failed to create job run: %#v", job))
    }

    if err := updateApp(ctx, cluster, app, params); err != nil {
        return apierror.InternalError(err, "updating application CR with staging information")
    }

    imageURL := params.ImageURL(params.RegistryURL)

    log.Info("staged app", "namespace", helmchart.Namespace(), "app", params.AppRef, "uid", uid, "image", imageURL)

    response.OKReturn(c, models.StageResponse{
        Stage:    models.NewStage(uid),
        ImageURL: imageURL,
    })
    return nil
}

// Staged handles the API endpoint /namespaces/:namespace/staging/:stage_id/complete
// It waits for the Job resource staging the app to complete
func Staged(c *gin.Context) apierror.APIErrors {
    ctx := c.Request.Context()

    namespace := c.Param("namespace")
    id := c.Param("stage_id")

    cluster, err := kubernetes.GetCluster(ctx)
    if err != nil {
        return apierror.InternalError(err)
    }

    // Wait for the staging to be done, then check if it ended in failure.
    // Select the job for this stage `id`.
    selector := fmt.Sprintf("app.kubernetes.io/component=staging,app.kubernetes.io/part-of=%s,epinio.io/stage-id=%s",
        namespace, id)

    jobList, err := cluster.ListJobs(ctx, helmchart.Namespace(), selector)
    if err != nil {
        return apierror.InternalError(err)
    }
    if len(jobList.Items) == 0 {
        return apierror.InternalError(fmt.Errorf("no jobs in %s with selector %s", namespace, selector))
    }

    for _, job := range jobList.Items {
        // Wait for job to be done
        err = cluster.WaitForJobDone(ctx, helmchart.Namespace(), job.Name, duration.ToAppBuilt())
        if err != nil {
            return apierror.InternalError(err)
        }
        // Check job for failure
        failed, err := cluster.IsJobFailed(ctx, job.Name, helmchart.Namespace())
        if err != nil {
            return apierror.InternalError(err)
        }
        if failed {
            return apierror.NewInternalError("Failed to stage",
                fmt.Sprintf("stage-id = %s", id))
        }
    }

    response.OK(c)
    return nil
}

func validateBlob(ctx context.Context, blobUID string, app models.AppRef, s3ConnectionDetails s3manager.ConnectionDetails) apierror.APIErrors {

    manager, err := s3manager.New(s3ConnectionDetails)
    if err != nil {
        return apierror.InternalError(err, "creating an S3 manager")
    }

    blobMeta, err := manager.Meta(ctx, blobUID)
    if err != nil {
        return apierror.InternalError(err, "querying blob id meta-data")
    }

    blobApp, ok := blobMeta["App"]
    if !ok {
        return apierror.NewInternalError("blob has no app name meta data")
    }
    if blobApp != app.Name {
        return apierror.NewBadRequestError("blob app mismatch").
            WithDetailsf("expected: [%s], found: [%s]", app.Name, blobApp)
    }

    blobNamespace, ok := blobMeta["Namespace"]
    if !ok {
        return apierror.NewInternalError("blob has no namespace meta data")
    }
    if blobNamespace != app.Namespace {
        return apierror.NewBadRequestError("blob namespace mismatch").
            WithDetailsf("expected: [%s], found: [%s]", app.Namespace, blobNamespace)
    }

    return nil
}

// newJobRun is a helper which creates the Job related resources from
// the given staging params. That is the job itself, and a secret
// holding the job's environment. Which is a copy of the app
// environment + standard variables.
func newJobRun(app stageParam) (*batchv1.Job, *corev1.Secret) {
    jobName := names.GenerateResourceName("stage", app.Namespace, app.Name, app.Stage.ID)

    // fake stage params of the previous to pull the old image url from.
    previous := app
    previous.Stage = models.NewStage(app.PreviousStageID)

    // TODO: Simplify env setup -- https://github.com/epinio/epinio/issues/1176
    // Note: `source` is required because the mounted files are not executable.

    // runtime: AWSCLIImage
    awsScript := fmt.Sprintf("source /stage-support/%s", helmchart.EpinioStageDownload)

    // runtime: BashImage
    unpackScript := fmt.Sprintf(`source /stage-support/%s`, helmchart.EpinioStageUnpack)

    // runtime: app.BuilderImage
    buildpackScript := fmt.Sprintf(`source /stage-support/%s`, helmchart.EpinioStageBuild)

    // build configuration
    // - shared between all the phases, even if each phase uses only part of the set
    stageEnv := assembleStageEnv(app, previous)

    volumeMounts := []corev1.VolumeMount{
        {
            Name:      "source",
            SubPath:   "source",
            MountPath: "/workspace/source",
        },
        {
            Name:      "cache",
            SubPath:   "cache",
            MountPath: "/workspace/cache",
        },
        {
            Name:      "registry-creds",
            MountPath: "/home/cnb/.docker/",
            ReadOnly:  true,
        },
        {
            Name:      "staging",
            MountPath: "/stage-support",
        },
        {
            Name:      "app-environment",
            MountPath: "/workspace/source/appenv",
            ReadOnly:  true,
        },
    }

    // mount AWS credentials secret only if the credentials are provided
    if app.S3ConnectionDetails.AccessKeyID != "" && app.S3ConnectionDetails.SecretAccessKey != "" {
        volumeMounts = append(volumeMounts, corev1.VolumeMount{
            Name:      "s3-creds",
            MountPath: "/root/.aws",
            ReadOnly:  true,
        })
    }

    // Cache Volume
    CacheVolumeSource := corev1.VolumeSource{
        PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
            ClaimName: app.MakeCachePVCName(),
            ReadOnly:  false,
        },
    }
    if app.HelmValues.Storage.Cache.EmptyDir {
        CacheVolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
    }

    // Source Blobs Volume
    SourceBlobsVolumeSource := corev1.VolumeSource{
        PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
            ClaimName: app.MakeSourceBlobsPVCName(),
            ReadOnly:  false,
        },
    }
    if app.HelmValues.Storage.SourceBlobs.EmptyDir {
        SourceBlobsVolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
    }


    volumes := []corev1.Volume{
        {
            Name: "staging",
            VolumeSource: corev1.VolumeSource{
                ConfigMap: &corev1.ConfigMapVolumeSource{
                    LocalObjectReference: corev1.LocalObjectReference{
                        Name: app.Scripts,
                    },
                    DefaultMode: ptr.To[int32](420),
                },
            },
        },
        {
            // See `jobenv` for the Secret providing the information.
            Name: "app-environment",
            VolumeSource: corev1.VolumeSource{
                Secret: &corev1.SecretVolumeSource{
                    SecretName:  jobName,
                    DefaultMode: ptr.To[int32](420),
                },
            },
        },
        {
            Name: "cache",
            VolumeSource: CacheVolumeSource,
        },
        {
            Name: "source",
            VolumeSource: SourceBlobsVolumeSource,
        },
        {
            Name: "s3-creds",
            VolumeSource: corev1.VolumeSource{
                Secret: &corev1.SecretVolumeSource{
                    SecretName:  helmchart.S3ConnectionDetailsSecretName,
                    DefaultMode: ptr.To[int32](420),
                },
            },
        },
        {
            Name: "registry-creds",
            VolumeSource: corev1.VolumeSource{
                Secret: &corev1.SecretVolumeSource{
                    SecretName:  registry.CredentialsSecretName,
                    DefaultMode: ptr.To[int32](420),
                    Items: []corev1.KeyToPath{
                        {
                            Key:  ".dockerconfigjson",
                            Path: "config.json",
                        },
                    },
                },
            },
        },
    }

    volumes, volumeMounts = mountS3Certs(volumes, volumeMounts)
    volumes, volumeMounts = mountRegistryCerts(app, volumes, volumeMounts)

    // Create job environment as a copy of the app environment
    env := make(map[string][]byte)
    for _, ev := range app.Environment {
        env[ev.Name] = []byte(ev.Value)
    }

    jobenv := &corev1.Secret{
        Data: env,
        ObjectMeta: metav1.ObjectMeta{
            Name: jobName,
            Labels: map[string]string{
                "app.kubernetes.io/name":       app.Name,
                "app.kubernetes.io/part-of":    app.Namespace,
                models.EpinioStageIDLabel:      app.Stage.ID,
                models.EpinioStageIDPrevious:   app.PreviousStageID,
                models.EpinioStageBlobUIDLabel: app.BlobUID,
                "app.kubernetes.io/managed-by": "epinio",
                "app.kubernetes.io/component":  "staging",
            },
            Annotations: map[string]string{
                models.EpinioCreatedByAnnotation: app.Username,
            },
        },
    }

    job := &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name: jobName,
            Labels: map[string]string{
                "app.kubernetes.io/name":       app.Name,
                "app.kubernetes.io/part-of":    app.Namespace,
                models.EpinioStageIDLabel:      app.Stage.ID,
                models.EpinioStageIDPrevious:   app.PreviousStageID,
                models.EpinioStageBlobUIDLabel: app.BlobUID,
                "app.kubernetes.io/managed-by": "epinio",
                "app.kubernetes.io/component":  "staging",
            },
            Annotations: map[string]string{
                models.EpinioCreatedByAnnotation: app.Username,
            },
        },
        Spec: batchv1.JobSpec{
            BackoffLimit: ptr.To[int32](0),
            TTLSecondsAfterFinished: getTTLPointer(app.HelmValues.TTLSecondsAfterFinished),
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: map[string]string{
                        "app.kubernetes.io/name":       app.Name,
                        "app.kubernetes.io/part-of":    app.Namespace,
                        models.EpinioStageIDLabel:      app.Stage.ID,
                        models.EpinioStageIDPrevious:   app.PreviousStageID,
                        models.EpinioStageBlobUIDLabel: app.BlobUID,
                        "app.kubernetes.io/managed-by": "epinio",
                        "app.kubernetes.io/component":  "staging",
                    },
                    Annotations: map[string]string{
                        // Allow communication with the Registry even before the proxy is ready
                        "config.linkerd.io/skip-outbound-ports": "443",
                        models.EpinioCreatedByAnnotation:        app.Username,
                    },
                },
                Spec: corev1.PodSpec{
                    ServiceAccountName: app.HelmValues.ServiceAccountName,
                    InitContainers: []corev1.Container{
                        {
                            Name:         "download-s3-blob",
                            Image:        app.DownloadImage,
                            VolumeMounts: volumeMounts,
                            Command:      []string{"/bin/bash"},
                            Args: []string{
                                "-c",
                                awsScript,
                            },
                            Env: stageEnv,
                        },
                        {
                            Name:         "unpack-blob",
                            Image:        app.UnpackImage,
                            VolumeMounts: volumeMounts,
                            Command:      []string{"bash"},
                            Args: []string{
                                "-c",
                                unpackScript,
                            },
                            Env: stageEnv,
                        },
                    },
                    Containers: []corev1.Container{
                        {
                            Name:    "buildpack",
                            Image:   app.BuilderImage,
                            Command: []string{"/bin/bash"},
                            Args: []string{
                                "-c",
                                buildpackScript,
                            },
                            Env:          stageEnv,
                            VolumeMounts: volumeMounts,
                            SecurityContext: &corev1.SecurityContext{
                                RunAsUser:  ptr.To[int64](app.UserID),
                                RunAsGroup: ptr.To[int64](app.GroupID),
                            },
                            Resources:      app.HelmValues.Resources,
                        },
                    },
                    RestartPolicy: corev1.RestartPolicyNever,
                    Volumes:       volumes,
                    Tolerations:   app.HelmValues.Tolerations,
                    NodeSelector:  app.HelmValues.NodeSelector,
                    Affinity:      getAffinityPointer(app.HelmValues.Affinity),
                },
            },
        },
    }

    return job, jobenv
}

func assembleStageEnv(app, previous stageParam) []corev1.EnvVar {
    stageEnv := []corev1.EnvVar{}

    protocol := "http"
    if app.S3ConnectionDetails.UseSSL {
        protocol = "https"
    }

    stageEnv = appendEnvVar(stageEnv, "PROTOCOL", protocol)
    stageEnv = appendEnvVar(stageEnv, "ENDPOINT", app.S3ConnectionDetails.Endpoint)
    stageEnv = appendEnvVar(stageEnv, "BUCKET", app.S3ConnectionDetails.Bucket)
    stageEnv = appendEnvVar(stageEnv, "BLOBID", app.BlobUID)
    stageEnv = appendEnvVar(stageEnv, "PREIMAGE", previous.ImageURL(previous.RegistryURL))
    stageEnv = appendEnvVar(stageEnv, "APPIMAGE", app.ImageURL(app.RegistryURL))
    stageEnv = appendEnvVar(stageEnv, "USERID", strconv.FormatInt(app.UserID, 10))
    stageEnv = appendEnvVar(stageEnv, "GROUPID", strconv.FormatInt(app.GroupID, 10))

    return stageEnv
}


func getRegistryURL(ctx context.Context, cluster *kubernetes.Cluster) (string, error) {
    cd, err := registry.GetConnectionDetails(ctx, cluster, helmchart.Namespace(), registry.CredentialsSecretName)
    if err != nil {
        return "", err
    }
    registryPublicURL, err := cd.PublicRegistryURL()
    if err != nil {
        return "", err
    }
    if registryPublicURL == "" {
        return "", errors.New("no public registry URL found")
    }

    return fmt.Sprintf("%s/%s", registryPublicURL, cd.Namespace), nil
}

// The equivalent of:
// kubectl get secret -n (helmchart.Namespace()) epinio-registry-tls -o json | jq -r '.["data"]["tls.crt"]' | base64 -d | openssl x509 -hash -noout
// written in golang.
func getRegistryCertificateHash(ctx context.Context, c *kubernetes.Cluster, namespace string, name string) (string, error) {
    secret, err := c.Kubectl.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        return "", err
    }

    // cert-manager doesn't add the CA for ACME certificates:
    // https://github.com/jetstack/cert-manager/issues/2111
    if _, found := secret.Data["tls.crt"]; !found {
        return "", nil
    }

    hash, err := cahash.GenerateHash(secret.Data["tls.crt"])
    if err != nil {
        return "", err
    }

    return hash, nil
}

// getBuilderImage returns the builder image defined on the request. If that
// one is not defined, it tries to find the builder image previously used on the
// Application CR. If one is not found, it returns an error.
func getBuilderImage(req models.StageRequest, app *unstructured.Unstructured) (string, apierror.APIErrors) {
    var returnErr apierror.APIErrors

    if req.BuilderImage != "" {
        return req.BuilderImage, nil
    }

    builderImage, _, err := unstructured.NestedString(app.UnstructuredContent(), "spec", "builderimage")
    if err != nil {
        returnErr = apierror.InternalError(err, "builderimage should be a string!")
        return "", returnErr
    }

    return builderImage, nil
}

func getBlobUID(ctx context.Context, s3ConnectionDetails s3manager.ConnectionDetails, req models.StageRequest, app *unstructured.Unstructured) (string, apierror.APIErrors) {
    var blobUID string
    var err error
    var returnErr apierror.APIErrors

    if req.BlobUID != "" {
        blobUID = req.BlobUID
    } else {
        blobUID, err = findPreviousBlobUID(app)
        if err != nil {
            returnErr = apierror.InternalError(err, "looking up the previous blod UID")
            return "", returnErr
        }
    }

    if blobUID == "" {
        returnErr = apierror.NewBadRequestError("request didn't provide a blobUID and a previous one doesn't exist")
        return "", returnErr
    }

    // Validate incoming blob id before attempting to stage
    apierr := validateBlob(ctx, blobUID, req.App, s3ConnectionDetails)
    if apierr != nil {
        return "", apierr
    }

    return blobUID, nil
}

func findPreviousBlobUID(app *unstructured.Unstructured) (string, error) {
    blobUID, _, err := unstructured.NestedString(app.UnstructuredContent(), "spec", "blobuid")
    if err != nil {
        return "", errors.New("blobuid should be string")
    }

    return blobUID, nil
}

func updateApp(ctx context.Context, cluster *kubernetes.Cluster, app *unstructured.Unstructured, params stageParam) error {
    if err := unstructured.SetNestedField(app.Object, params.BlobUID, "spec", "blobuid"); err != nil {
        return err
    }
    if err := unstructured.SetNestedField(app.Object, params.Stage.ID, "spec", "stageid"); err != nil {
        return err
    }
    if err := unstructured.SetNestedField(app.Object, params.BuilderImage, "spec", "builderimage"); err != nil {
        return err
    }

    client, err := cluster.ClientApp()
    if err != nil {
        return err
    }

    namespace, _, err := unstructured.NestedString(app.UnstructuredContent(), "metadata", "namespace")
    if err != nil {
        return err
    }

    _, err = client.Namespace(namespace).Update(ctx, app, metav1.UpdateOptions{})

    return err
}

func mountS3Certs(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount) {
    if s3CertificateSecret := viper.GetString("s3-certificate-secret"); s3CertificateSecret != "" {
        volumes = append(volumes, corev1.Volume{
            Name: "s3-certs",
            VolumeSource: corev1.VolumeSource{
                Secret: &corev1.SecretVolumeSource{
                    SecretName:  s3CertificateSecret,
                    DefaultMode: ptr.To[int32](420),
                },
            },
        })

        volumeMounts = append(volumeMounts, corev1.VolumeMount{
            Name:      "s3-certs",
            MountPath: "/certs",
            ReadOnly:  true,
        })
    }

    return volumes, volumeMounts
}

func mountRegistryCerts(app stageParam, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount) {
    // If there is a certificate to trust
    if app.RegistryCASecret != "" && app.RegistryCAHash != "" {
        volumes = append(volumes, corev1.Volume{
            Name: "registry-certs",
            VolumeSource: corev1.VolumeSource{
                Secret: &corev1.SecretVolumeSource{
                    SecretName:  app.RegistryCASecret,
                    DefaultMode: ptr.To[int32](420),
                },
            },
        })

        volumeMounts = append(volumeMounts, corev1.VolumeMount{
            Name:      "registry-certs",
            MountPath: fmt.Sprintf("/etc/ssl/certs/%s", app.RegistryCAHash),
            SubPath:   "tls.crt",
            ReadOnly:  true,
        })
    }

    return volumes, volumeMounts
}

func appendEnvVar(envs []corev1.EnvVar, name, value string) []corev1.EnvVar {
    return append(envs, corev1.EnvVar{Name: name, Value: value})
}

// StagingScriptConfig holds all the information for using a (set of) buildpack(s)
type StagingScriptConfig struct {
    Name          string                // config name. Needed to mount the resource in the pod
    Builder       string                // glob pattern for builders supported by this resource
    UserID        int64                 // user id to run the build phase with (`cnb` user)
    GroupID       int64                 // group id to run the build hase with
    Base          string                // optional, name of resource to pull the other parts from
    DownloadImage string                // image to run the download phase with
    UnpackImage   string                // image to run the unpack phase with
    Env           models.EnvVariableMap // environment settings
    HelmValues    HelmValuesMap         // Helm Values configuring the staging workload
}

func DetermineStagingScripts(ctx context.Context,
    logger logr.Logger,
    cluster *kubernetes.Cluster,
    namespace, builder string) (*StagingScriptConfig, error) {

    logger.Info("locate staging scripts", "namespace", namespace)
    logger.Info("locate staging scripts", "builder", builder)

    configmapSelector := labels.Set(map[string]string{
        "app.kubernetes.io/component": "epinio-staging",
    }).AsSelector().String()

    configmapList, err := cluster.Kubectl.CoreV1().ConfigMaps(namespace).List(ctx,
        metav1.ListOptions{
            LabelSelector: configmapSelector,
        })
    if err != nil {
        return nil, err
    }

    logger.Info("locate staging scripts", "possibles", len(configmapList.Items))

    var candidates []*StagingScriptConfig
    for _, configmap := range configmapList.Items {
        config, err := NewStagingScriptConfig(configmap)
        if err != nil {
            logger.Info("Error loading config item", fmt.Sprintf("%+v", err))
            return nil, err
        }
        candidates = append(candidates, config)
    }

    // Sort the candidates by name to have a deterministic search order
    slices.SortFunc(candidates, SortByName)

    var fallback *StagingScriptConfig

    // Show the entire ordered list (except fallbacks), also save fallbacks, and show last. NOTE
    // that this loop excises the fallbacks from the regular list to avoid having to check for
    // them again when matching.
    var matchable []*StagingScriptConfig
    for _, config := range candidates {
        pattern := config.Builder
        if pattern == "*" || pattern == "" {
            fallback = config
            continue
        }
        logger.Info("locate staging scripts - found",
            "name", config.Name, "match", pattern)
        matchable = append(matchable, config)
    }
    if fallback != nil {
        logger.Info("locate staging scripts - fallback",
            "name", fallback.Name, "match", fallback.Builder)
    }

    // match by pattern
    for _, config := range matchable {
        pattern := config.Builder
        logger.Info("locate staging scripts - checking",
            "name", config.Name, "match", pattern)

        matched, err := filepath.Match(pattern, builder)
        if err != nil {
            return nil, err
        }
        if matched {
            logger.Info("locate staging scripts - match",
                "name", config.Name, "match", pattern, "builder", builder)

            err := StagingScriptConfigResolve(ctx, logger, cluster, config)
            if err != nil {
                return nil, err
            }

            return config, nil
        }
    }

    if fallback != nil {
        logger.Info("locate staging scripts - using fallback", "name", fallback.Name)
        err := StagingScriptConfigResolve(ctx, logger, cluster, fallback)
        if err != nil {
            return nil, err
        }
        return fallback, nil
    }

    return nil, fmt.Errorf("no matches, no fallback")
}

func StagingScriptConfigResolve(ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster, config *StagingScriptConfig) error {
    if config.Base == "" {
        // nothing to do without a base
        return nil
    }

    logger.Info("locate staging scripts - inherit", "base", config.Base)

    base, err := cluster.GetConfigMap(ctx, helmchart.Namespace(), config.Base)
    if err != nil {
        return apierror.InternalError(err, "failed to retrieve staging base")
    }

    // Fill config from the base.
    // BEWARE: Keep user/group data of the incoming config.
    // BEWARE: Keep environment data of the incoming config.

    config.Name = base.Name
    config.DownloadImage = base.Data["downloadImage"]
    config.UnpackImage = base.Data["unpackImage"]

    return nil
}

func NewStagingScriptConfig(config corev1.ConfigMap) (*StagingScriptConfig, error) {
    stagingScript := &StagingScriptConfig{
        Name:          config.Name,
        Builder:       config.Data["builder"],
        Base:          config.Data["base"],
        DownloadImage: config.Data["downloadImage"],
        UnpackImage:   config.Data["unpackImage"],
        // env, user, group id, Helm Values, see below.
    }

    userID, err := strconv.ParseInt(config.Data["userID"], 10, 64)
    if err != nil {
        return nil, apierror.InternalError(err)
    }
    groupID, err := strconv.ParseInt(config.Data["groupID"], 10, 64)
    if err != nil {
        return nil, err
    }

    envString := config.Data["env"]

    err = yaml.Unmarshal([]byte(envString), &stagingScript.Env)
    if err != nil {
        return nil, err
    }


    var cfg HelmValuesMap
    stagingValues,ok := config.Data["staging-values.json"]
    if ok {
        err = json.Unmarshal([]byte(stagingValues), &cfg)
        if err != nil {
            return nil, err
        }
        stagingScript.HelmValues = cfg
    }

    stagingScript.GroupID = groupID
    stagingScript.UserID = userID

    return stagingScript, nil
}

func SortByName(s1, s2 *StagingScriptConfig) int {
    return strings.Compare(s1.Name, s2.Name)
}
