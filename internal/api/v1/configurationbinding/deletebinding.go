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

package configurationbinding

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

func DeleteBinding(ctx context.Context, cluster *kubernetes.Cluster, namespace, appName, username string, configurationNames []string) apierror.APIErrors {

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	err = application.BoundConfigurationsUnset(ctx, cluster, app.Meta, configurationNames)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app.Workload != nil {
		_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, username, "")
		if apierr != nil {
			return apierr
		}
	}

	return nil
}
