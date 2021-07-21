package testenv

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/helpers"

	. "github.com/onsi/gomega"
)

func SetupInClusterServices(epinioBinary string) {
	out, err := proc.Run(fmt.Sprintf("%s%s enable services-incluster", Root(), epinioBinary), "", false)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(ContainSubstring("Beware, "))

	// Wait until classes appear
	EventuallyWithOffset(1, func() error {
		_, err = helpers.Kubectl("get clusterserviceclass mariadb")
		return err
	}, "5m").ShouldNot(HaveOccurred())

	// Wait until plans appear
	EventuallyWithOffset(1, func() error {
		_, err = helpers.Kubectl("get clusterserviceplan mariadb-10-3-22")
		return err
	}, "5m").ShouldNot(HaveOccurred())
}
