// Copyright © 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package instance manages the persistent Epinio installation identity.
package instance

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	dataKeyID            = "id"
	dataKeyInstallMethod = "installMethod"

	InstallMethodHelm    = "helm"
	InstallMethodCLI     = "cli"
	InstallMethodUnknown = "unknown"
)

// Info is the persistent identity of an Epinio installation.
type Info struct {
	ID            string
	InstallMethod string
}

// cached holds the instance identity after a successful GetOrCreate.
// /info is unauthenticated and can be high-traffic; avoid a live API call per request.
var cached atomic.Pointer[Info]

// GetCached returns the in-memory instance identity when available.
func GetCached() (Info, bool) {
	if info := cached.Load(); info != nil && info.ID != "" {
		return *info, true
	}
	return Info{}, false
}

// GetCachedOrCreate returns the cached identity, loading/creating it once when needed.
func GetCachedOrCreate(ctx context.Context, cluster *kubernetes.Cluster) (Info, error) {
	if info, ok := GetCached(); ok {
		return info, nil
	}
	return GetOrCreate(ctx, cluster)
}

// GetOrCreate returns the persistent instance identity, creating the
// ConfigMap when missing (upgrade path from older installs) or when the
// stored id is empty. An existing id is never replaced.
func GetOrCreate(ctx context.Context, cluster *kubernetes.Cluster) (Info, error) {
	namespace := helmchart.Namespace()
	cm, err := cluster.GetConfigMap(ctx, namespace, helmchart.EpinioInstanceConfigMapName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return Info{}, errors.Wrap(err, "getting epinio-instance ConfigMap")
		}
		info, createErr := create(ctx, cluster, namespace)
		if createErr == nil {
			storeCache(info)
		}
		return info, createErr
	}

	info := fromConfigMap(cm)
	if info.ID != "" && info.InstallMethod != "" {
		storeCache(info)
		return info, nil
	}

	// Backfill missing fields. Retry on conflict in case another replica
	// updates the ConfigMap at the same time (one-time upgrade path).
	var result Info
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, getErr := cluster.GetConfigMap(ctx, namespace, helmchart.EpinioInstanceConfigMapName)
		if getErr != nil {
			return getErr
		}
		result = fromConfigMap(current)
		changed := false
		if result.ID == "" {
			result.ID = uuid.NewString()
			changed = true
		}
		if result.InstallMethod == "" {
			result.InstallMethod = defaultInstallMethod()
			changed = true
		}
		if !changed {
			return nil
		}
		if current.Data == nil {
			current.Data = map[string]string{}
		}
		current.Data[dataKeyID] = result.ID
		current.Data[dataKeyInstallMethod] = result.InstallMethod
		_, updateErr := cluster.Kubectl.CoreV1().ConfigMaps(namespace).Update(ctx, current, metav1.UpdateOptions{})
		return updateErr
	})
	if err != nil {
		return Info{}, errors.Wrap(err, "updating epinio-instance ConfigMap")
	}
	storeCache(result)
	return result, nil
}

func create(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (Info, error) {
	info := Info{
		ID:            uuid.NewString(),
		InstallMethod: defaultInstallMethod(),
	}

	// Labels must match the Helm chart's epinio.labels (static parts) so a
	// later chart upgrade that adopts this ConfigMap does not conflict.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helmchart.EpinioInstanceConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component": "epinio",
				"app.kubernetes.io/instance":  "default",
				"app.kubernetes.io/name":      "epinio-server",
				"app.kubernetes.io/part-of":   "epinio",
				"epinio.io/instance-metadata": "true",
			},
		},
		Data: map[string]string{
			dataKeyID:            info.ID,
			dataKeyInstallMethod: info.InstallMethod,
		},
	}

	_, err := cluster.Kubectl.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			existing, getErr := cluster.GetConfigMap(ctx, namespace, helmchart.EpinioInstanceConfigMapName)
			if getErr != nil {
				return Info{}, errors.Wrap(getErr, "re-reading epinio-instance ConfigMap after create race")
			}
			return fromConfigMap(existing), nil
		}
		return Info{}, errors.Wrap(err, "creating epinio-instance ConfigMap")
	}
	return info, nil
}

func fromConfigMap(cm *corev1.ConfigMap) Info {
	if cm == nil || cm.Data == nil {
		return Info{}
	}
	return Info{
		ID:            strings.TrimSpace(cm.Data[dataKeyID]),
		InstallMethod: strings.TrimSpace(cm.Data[dataKeyInstallMethod]),
	}
}

func defaultInstallMethod() string {
	method := strings.TrimSpace(strings.ToLower(viper.GetString("install-method")))
	switch method {
	case InstallMethodCLI:
		return InstallMethodCLI
	case InstallMethodHelm, "":
		// Helm is the only in-tree install path; empty means chart/env not set yet.
		return InstallMethodHelm
	default:
		return method
	}
}

func storeCache(info Info) {
	cp := info
	cached.Store(&cp)
}

// ResetCache clears the in-memory cache. Intended for tests.
func ResetCache() {
	cached.Store(nil)
}
