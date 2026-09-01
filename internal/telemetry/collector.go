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

package telemetry

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/instance"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/internal/version"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
)

// Collect gathers fleet inventory from the cluster.
func Collect(ctx context.Context, cfg Config) (models.FleetInventory, error) {
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return models.FleetInventory{}, errors.Wrap(err, "getting kubernetes cluster")
	}

	kubeVersion, err := cluster.GetVersion()
	if err != nil {
		return models.FleetInventory{}, errors.Wrap(err, "getting kubernetes version")
	}

	nsList, err := namespaces.List(ctx, cluster)
	if err != nil {
		return models.FleetInventory{}, errors.Wrap(err, "listing namespaces")
	}

	appRefs, err := application.ListAppRefs(ctx, cluster, "")
	if err != nil {
		return models.FleetInventory{}, errors.Wrap(err, "listing applications")
	}

	serviceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return models.FleetInventory{}, errors.Wrap(err, "creating service client")
	}

	serviceList, err := serviceClient.ListAll(ctx)
	if err != nil {
		return models.FleetInventory{}, errors.Wrap(err, "listing services")
	}

	instanceInfo, err := instance.GetCachedOrCreate(ctx, cluster)
	if err != nil {
		return models.FleetInventory{}, errors.Wrap(err, "loading instance identity")
	}

	return models.FleetInventory{
		InstanceID:         instanceInfo.ID,
		InstallMethod:      instanceInfo.InstallMethod,
		Cluster:            cfg.Cluster,
		Environment:        cfg.Environment,
		EpinioVersion:      version.Version,
		EpinioChartVersion: version.ChartVersion,
		KubernetesVersion:  kubeVersion,
		Platform:           cluster.GetPlatform().String(),
		ApplicationCount:   len(appRefs),
		NamespaceCount:     len(nsList),
		ServiceCount:       len(serviceList),
	}, nil
}
