package domain

import (
	"context"
	"fmt"
	"strings"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/pkg/errors"
)

var mainDomain = ""

func AppDefaultRoute(ctx context.Context, name string) (string, error) {
	mainDomain, err := MainDomain(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", name, mainDomain), nil
}

// GetMain finds the main domain for Epinio
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
