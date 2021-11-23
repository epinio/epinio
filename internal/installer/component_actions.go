package installer

import (
	"context"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/duration"
	"github.com/go-logr/logr"
)

type ComponentActions struct {
	cluster *kubernetes.Cluster
	ui      *termui.UI
	log     logr.Logger
	timeout time.Duration
}

// NewComponentActions returns the runner for component actions, like checks and waitFors
func NewComponentActions(ui *termui.UI, cluster *kubernetes.Cluster, log logr.Logger, timeout time.Duration) *ComponentActions {
	return &ComponentActions{
		ui:      ui,
		cluster: cluster,
		log:     log,
		timeout: timeout,
	}
}

func (ca ComponentActions) Run(ctx context.Context, c Component, chk ComponentAction) error {
	namespace := c.Namespace
	if chk.Namespace != "" {
		namespace = chk.Namespace
	}
	switch chk.Type {
	case Pod:
		if err := ca.cluster.WaitForPodBySelector(ctx, ca.ui, namespace, chk.Selector, duration.ToPodReady()); err != nil {
			return err
		}
	case Loadbalancer:
		return ca.cluster.WaitUntilServiceHasLoadBalancer(ctx, ca.ui, namespace, chk.Selector, duration.ToServiceLoadBalancer())
	case CRD:
		return ca.cluster.WaitForCRD(ctx, ca.ui, chk.Selector, ca.timeout)
	case Job:
		if err := ca.cluster.WaitForJobCompleted(ctx, namespace, chk.Selector, ca.timeout); err != nil {
			return err
		}
	}

	return nil
}
