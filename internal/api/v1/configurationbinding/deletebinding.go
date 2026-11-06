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
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// DeleteBinding unbinds the named configurations from the app. It does
// not redeploy: it returns the app, and the caller redeploys every
// affected app once (apiapp.RedeployIfRunning).
func DeleteBinding(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace, appName string,
	configurationNames []string,
) (*models.App, apierror.APIErrors) {
	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return nil, apierror.InternalError(err)
	}
	if app == nil {
		return nil, apierror.AppIsNotKnown(appName)
	}

	err = application.BoundConfigurationsUnset(
		ctx, cluster, app.Meta, configurationNames,
	)
	if err != nil {
		return nil, apierror.InternalError(err)
	}

	return app, nil
}
