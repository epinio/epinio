package kubernetes_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/kubernetes/kubernetesfakes"
)

type FakeReader struct {
	Values map[string]string
}

func (f *FakeReader) Read(opt InstallationOption) (interface{}, error) {
	return f.Values[opt.Name+"-"+string(opt.DeploymentID)], nil
}

var _ = Describe("Installer", func() {
	var installer *Installer
	var deployment1 = &kubernetesfakes.FakeDeployment{}
	var deployment2 = &kubernetesfakes.FakeDeployment{}

	Describe("GatherNeededOptions", func() {
		BeforeEach(func() {
			deployment1.NeededOptionsReturns(
				InstallationOptions{
					{Name: "SharedOption", DeploymentID: "", Value: "ShouldBeIgnored"},
					{Name: "PrivateOption1", DeploymentID: "Deployment1"},
				})
			deployment2.NeededOptionsReturns(
				InstallationOptions{
					{Name: "SharedOption", DeploymentID: "", Value: "ShouldBeIgnoredToo"},
					{Name: "PrivateOption2", DeploymentID: "Deployment2"},
				})
			installer = NewInstaller(deployment1, deployment2)
		})

		It("returns a combination of all options from all deployments", func() {
			installer.GatherNeededOptions()
			Expect(len(installer.NeededOptions)).To(Equal(3))
			Expect(installer.NeededOptions).To(
				ContainElement(InstallationOption{Name: "SharedOption", Value: ""}))
			Expect(installer.NeededOptions).To(ContainElement(InstallationOption{
				Name:         "PrivateOption1",
				DeploymentID: "Deployment1",
			}))
			Expect(installer.NeededOptions).To(ContainElement(InstallationOption{
				Name:         "PrivateOption2",
				DeploymentID: "Deployment2",
			}))
		})

		It("ignores default values on shared options", func() {
			installer.GatherNeededOptions()
			Expect(installer.NeededOptions).To(ContainElement(
				InstallationOption{Name: "SharedOption", Value: ""}))
		})
	})

	Describe("PopulateNeededOptions", func() {
		BeforeEach(func() {
			deployment1.NeededOptionsReturns(
				InstallationOptions{
					{
						Name:         "SharedOption",
						DeploymentID: "",
						Value:        "", // To be filled in
					},
					{
						Name:         "PrivateOption1",
						DeploymentID: "Deployment1",
						Value:        "SomeDefault",
					},
				})
			deployment2.NeededOptionsReturns(
				InstallationOptions{
					{
						Name:         "SharedOption",
						DeploymentID: "",
						Value:        "Default",
					},
					{
						Name:         "PrivateOption2",
						DeploymentID: "Deployment2",
						Value:        "", // to be filled
					},
				})
			installer = NewInstaller(deployment1, deployment2)
			installer.GatherNeededOptions()
		})

		It("returns a combination of all options from all deployments", func() {
			fakereader := &FakeReader{
				Values: map[string]string{
					"SharedOption-":              "something-returned-by-user",
					"PrivateOption2-Deployment2": "something-returned-by-user-private2",
				},
			}
			installer.GatherNeededOptions()
			installer.PopulateNeededOptions(fakereader)
			Expect(len(installer.NeededOptions)).To(Equal(3))
			Expect(installer.NeededOptions).To(ContainElement(InstallationOption{
				Name:  "SharedOption",
				Value: "something-returned-by-user",
			}))
			Expect(installer.NeededOptions).To(ContainElement(InstallationOption{
				Name:         "PrivateOption2",
				DeploymentID: "Deployment2",
				Value:        "something-returned-by-user-private2",
			}))
		})
	})
})
