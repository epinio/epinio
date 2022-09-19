package machine

import (
	"github.com/epinio/epinio/acceptance/helpers/proc"
	. "github.com/onsi/ginkgo/v2"
)

func (m *Machine) ListCRDS() {
	By("List CRDs")
	crds, _ := proc.Kubectl("get", "crds")
	By("CRDs: " + crds)
}

func (m *Machine) SeeCRD(crd string) {
	By("See CRD:" + crd)
	crdDesc, _ := proc.Kubectl("get", "crds", crd, "-o", "yaml")
	By("Spec: " + crdDesc)
}

func (m *Machine) ListServiceCRS() {
	By("List Service Catalog CRs")
	crds, _ := proc.Kubectl("get", "services.application.epinio.io",
		"-n", "epinio")
	By("CRDs: " + crds)
}

func (m *Machine) SeeServiceCR(cr string) {
	By("See Service Catalog CR: " + cr)
	crDesc, _ := proc.Kubectl("get", "services.application.epinio.io",
		"-n", "epinio", cr, "-o", "yaml")
	By("CR: " + crDesc)
}
