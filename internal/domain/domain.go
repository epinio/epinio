// Package domain collects structures and functions around the domains
// the client works with.
package domain

import (
	"context"
	"fmt"
	"strings"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/pkg/errors"
)

// mainDomain is the memoization cache for the name of the main domain
// of the currently accessed epinio installation.
var mainDomain = ""

// AppDefaultRoute constructs and returns an application's default
// route from the main domain and the name of the application
func AppDefaultRoute(ctx context.Context, name string) (string, error) {
	mainDomain, err := MainDomain(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", name, mainDomain), nil
}

func EpinioRegistryPublicURL(ctx context.Context) (string, error) {
	// TODO: Either find the external registry or construct this here

	mainDomain, err := MainDomain(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%s/apps", deployments.RegistryDeploymentID, mainDomain), nil
}

// MainDomain determines the name of the main domain of the currently
// accessed epinio installation. The result is cached in-memory (see
// variable mainDomain). The function preferably returns cached data,
// and queries the cluster ingresses only the first time the data is
// asked for. This is especially useful for long running commands. In
// other other words, epinio's API server.
func MainDomain(ctx context.Context) (string, error) {
	if mainDomain != "" {
		return mainDomain, nil
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return "", err
	}

	// Get the epinio ingress
	ingresses, err := cluster.ListIngress(ctx, deployments.EpinioDeploymentID, "app.kubernetes.io/name=epinio")
	if err != nil {
		return "", errors.Wrap(err, "failed to list ingresses for epinio")
	}

	if len(ingresses.Items) < 1 {
		return "", errors.New("epinio ingress not found")
	}

	if len(ingresses.Items) > 1 {
		return "", errors.New("more than one epinio ingress found")
	}

	if len(ingresses.Items[0].Spec.Rules) < 1 {
		return "", errors.New("epinio ingress has no rules")
	}

	if len(ingresses.Items[0].Spec.Rules) > 1 {
		return "", errors.New("epinio ingress has more than on rule")
	}

	host := ingresses.Items[0].Spec.Rules[0].Host
	mainDomain := strings.TrimPrefix(host, "epinio.")

	return mainDomain, nil
}
