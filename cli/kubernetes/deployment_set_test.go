package kubernetes_test

import (
	// "bytes"
	// "io"
	// "os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/kubernetes/kubernetesfakes"
	// "github.com/suse/carrier/cli/paas/ui"
)

type FakeReader struct {
	Values map[string]string
}

func (f *FakeReader) Read(opt *InstallationOption) error {
	opt.Value = f.Values[opt.Name+"-"+string(opt.DeploymentID)]
	return nil
}

// // Snarfed from
// // https://stackoverflow.com/questions/10473800/in-go-how-do-i-capture-stdout-of-a-function-into-a-string/10476304#10476304,
// // with local adaptions.
// func captureStdout(f func()) string {
// 	orig := os.Stdout
// 	r, w, _ := os.Pipe()
// 	os.Stdout = w

// 	outC := make(chan string)
// 	// Copying the output is done in a separate goroutine. This
// 	// prevents printing from blocking indefinitely.
// 	go func() {
// 		var buf bytes.Buffer
// 		io.Copy(&buf, r)
// 		outC <- buf.String()
// 	}()

// 	f()

// 	// Restore original state
// 	w.Close()
// 	os.Stdout = orig

// 	out := <-outC
// 	return out
// }

var _ = Describe("DeploymentSet", func() {
	var deploymentSet *DeploymentSet
	// var cluster Cluster
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
			deploymentSet = NewDeploymentSet(deployment1, deployment2)
		})

		It("returns a combination of all options from all deployments", func() {
			deploymentSet.GatherNeededOptions()
			Expect(len(deploymentSet.NeededOptions)).To(Equal(3))
			Expect(deploymentSet.NeededOptions).To(
				ContainElement(InstallationOption{Name: "SharedOption", Value: ""}))
			Expect(deploymentSet.NeededOptions).To(ContainElement(InstallationOption{
				Name:         "PrivateOption1",
				DeploymentID: "Deployment1",
			}))
			Expect(deploymentSet.NeededOptions).To(ContainElement(InstallationOption{
				Name:         "PrivateOption2",
				DeploymentID: "Deployment2",
			}))
		})

		It("ignores default values on shared options", func() {
			deploymentSet.GatherNeededOptions()
			Expect(deploymentSet.NeededOptions).To(ContainElement(
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
			deploymentSet = NewDeploymentSet(deployment1, deployment2)
		})

		It("returns a combination of all options from all deployments", func() {
			fakereader := &FakeReader{
				Values: map[string]string{
					"SharedOption-":              "something-returned-by-user",
					"PrivateOption2-Deployment2": "something-returned-by-user-private2",
				},
			}
			deploymentSet.GatherNeededOptions()
			deploymentSet.PopulateNeededOptions(fakereader)
			Expect(len(deploymentSet.NeededOptions)).To(Equal(3))
			Expect(deploymentSet.NeededOptions).To(ContainElement(InstallationOption{
				Name:  "SharedOption",
				Value: "something-returned-by-user",
			}))
			Expect(deploymentSet.NeededOptions).To(ContainElement(InstallationOption{
				Name:         "PrivateOption2",
				DeploymentID: "Deployment2",
				Value:        "something-returned-by-user-private2",
			}))
		})
	})

	// Describe("ShowNeededOptions", func() {
	// 	BeforeEach(func() {
	// 		deploymentSet = NewDeploymentSet(deployment1, deployment2)
	// 		deploymentSet.NeededOptions = InstallationOptions{
	// 			{
	// 				Name:  "A string",
	// 				Value: "fake",
	// 				Type:  StringType,
	// 			},
	// 			{
	// 				Name:  "A flag",
	// 				Value: true,
	// 				Type:  BooleanType,
	// 			},
	// 			{
	// 				Name:  "A count",
	// 				Value: 77,
	// 				Type:  IntType,
	// 			},
	// 		}
	// 	})

	// 	It("prints the values of all options", func() {
	// 		output := captureStdout(func() {
	// 			deploymentSet.ShowNeededOptions()
	// 		})
	// 		Expect(string(output)).To(ContainSubstring("Configuration..."))
	// 		Expect(string(output)).To(ContainSubstring("A string:"))
	// 		Expect(string(output)).To(ContainSubstring("fake"))
	// 		Expect(string(output)).To(ContainSubstring("A flag:"))
	// 		Expect(string(output)).To(ContainSubstring("true"))
	// 		Expect(string(output)).To(ContainSubstring("A count:"))
	// 		Expect(string(output)).To(ContainSubstring("77"))
	// 	})
	// })

	// Describe("Install", func() {
	// 	BeforeEach(func() {
	// 		deployment1.DeployReturns(nil)
	// 		deployment2.DeployReturns(nil)
	// 		deploymentSet = NewDeploymentSet(deployment1, deployment2)
	// 		cluster = Cluster{}
	// 		deploymentSet.NeededOptions = InstallationOptions{
	// 			{
	// 				Name:         "Option1",
	// 				DeploymentID: "Deployment1",
	// 			},
	// 			{
	// 				Name:         "Option2",
	// 				DeploymentID: "Deployment2",
	// 			},
	// 			{
	// 				Name:         "Option3",
	// 				DeploymentID: "",
	// 			},
	// 		}
	// 	})

	// 	It("calls Deploy method on deployments", func() {
	// 		deployment1.IDReturns("Deployment1")
	// 		deployment2.IDReturns("Deployment2")
	// 		deploymentSet.UI = ui.NewUI()
	// 		deploymentSet.Install(&cluster)
	// 		Expect(deployment1.DeployCallCount()).To(Equal(1))
	// 		Expect(deployment2.DeployCallCount()).To(Equal(1))
	// 	})

	// 	It("calls Deploy method with the correct InstallationOptions for each deployment", func() {
	// 		deploymentSet.UI = ui.NewUI()
	// 		deploymentSet.Install(&cluster)
	// 		_, opts := deployment1.DeployArgsForCall(1)
	// 		Expect(opts).To(ContainElement(
	// 			InstallationOption{
	// 				Name:         "Option1",
	// 				DeploymentID: "Deployment1",
	// 			}))
	// 		Expect(opts).To(ContainElement(
	// 			InstallationOption{
	// 				Name:         "Option3",
	// 				DeploymentID: "",
	// 			}))
	// 		Expect(len(opts)).To(Equal(2))
	// 	})
	// })
})
