package kubernetes_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/suse/carrier/cli/kubernetes"
)

var _ = Describe("InstallationOption", func() {
	Describe("ToOptMapKey", func() {
		option := kubernetes.InstallationOption{
			Name:         "TheName",
			DeploymentID: "SomeDeployment",
		}
		It("returns a combination of Name + deploymentID", func() {
			Expect(option.ToOptMapKey()).To(Equal("TheName-SomeDeployment"))
		})
	})
})

var _ = Describe("InstallationOptions", func() {
	Describe("ToOptMap", func() {
		options := kubernetes.InstallationOptions{
			{
				Name:         "OptionName",
				Value:        "ForDeployment1",
				DeploymentID: "Deployment1",
			},
			{
				Name:         "OptionName",
				Value:        "ThisShouldBeLost",
				DeploymentID: "Deployment2",
			},
			{
				Name:         "OptionName",
				Value:        "ForDeployment2",
				DeploymentID: "Deployment2",
			},
			{
				Name:         "OptionName",
				Value:        "ForAllDeployments",
				DeploymentID: "",
			},
		}
		It("returns a map matching the InstallationOptions", func() {
			optMap := options.ToOptMap()
			Expect(optMap["OptionName-Deployment1"].Value).To(Equal("ForDeployment1"))
			Expect(optMap["OptionName-Deployment2"].Value).To(Equal("ForDeployment2"))
			Expect(optMap["OptionName-"].Value).To(Equal("ForAllDeployments"))
		})
	})

	Describe("Merge", func() {
		When("merging shared options", func() {
			var sharedOption, privateOption kubernetes.InstallationOption
			var installationOptions kubernetes.InstallationOptions
			BeforeEach(func() {
				sharedOption = kubernetes.InstallationOption{
					Name:         "Option",
					Value:        "the old value",
					DeploymentID: "", // This is what makes it shared
				}
				privateOption = kubernetes.InstallationOption{
					Name:         "Option",
					Value:        "private value",
					DeploymentID: "MyDeploymentID", // This is what makes it private
				}
				installationOptions = kubernetes.InstallationOptions{sharedOption, privateOption}
			})
			It("returns only one instance of the shared option", func() {
				result := installationOptions.Merge(kubernetes.InstallationOptions{
					{Name: "Option", Value: "the new value", DeploymentID: ""},
				})
				Expect(result.GetString("Option", "")).To(Equal("the new value"))
			})

			It("doesn't overwrite private options with shared ones", func() {
				result := installationOptions.Merge(kubernetes.InstallationOptions{
					{Name: "Option", Value: "the new value", DeploymentID: ""},
				})
				Expect(result.GetString("Option", "MyDeploymentID")).To(Equal("private value"))
			})

			It("Returns every instance of private options (even when name match)", func() {
				result := installationOptions.Merge(kubernetes.InstallationOptions{
					{Name: "Option", Value: "the new value", DeploymentID: "OtherDeploymentID"},
				})
				Expect(result.GetString("Option", "MyDeploymentID")).To(Equal("private value"))
				Expect(result.GetString("Option", "OtherDeploymentID")).To(Equal("the new value"))
			})
		})
	})

	Describe("GetString", func() {
		var options kubernetes.InstallationOptions
		When("option is a string", func() {
			BeforeEach(func() {
				options = kubernetes.InstallationOptions{
					kubernetes.InstallationOption{
						Name:  "Option",
						Value: "the value",
						Type:  kubernetes.StringType,
					},
				}
			})
			It("returns a string value", func() {
				result, err := options.GetString("Option", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal("the value"))
			})
		})
		When("option is not a string", func() {
			BeforeEach(func() {
				options = kubernetes.InstallationOptions{
					kubernetes.InstallationOption{
						Name:  "Option",
						Value: true,
						Type:  kubernetes.BooleanType,
					},
				}
			})
			It("panics", func() {
				Expect(func() { options.GetString("Option", "") }).
					To(PanicWith(MatchRegexp("wrong type assertion")))
			})
		})

		When("option doesn't exist", func() {
			BeforeEach(func() {
				options = kubernetes.InstallationOptions{}
			})
			It("returns an error", func() {
				_, err := options.GetString("Option", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
			})
		})
	})

	Describe("GetInt", func() {
		var options kubernetes.InstallationOptions
		When("option is an int", func() {
			BeforeEach(func() {
				options = kubernetes.InstallationOptions{
					kubernetes.InstallationOption{
						Name:  "Option",
						Value: 3,
						Type:  kubernetes.IntType,
					},
				}
			})
			It("returns an int value", func() {
				result, err := options.GetInt("Option", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(3))
			})
		})

		When("option is not an int", func() {
			BeforeEach(func() {
				options = kubernetes.InstallationOptions{
					kubernetes.InstallationOption{
						Name:  "Option",
						Value: true,
						Type:  kubernetes.BooleanType,
					},
				}
			})
			It("panics", func() {
				Expect(func() { options.GetInt("Option", "") }).
					To(PanicWith(MatchRegexp("wrong type assertion")))
			})
		})

		When("option doesn't exist", func() {
			BeforeEach(func() {
				options = kubernetes.InstallationOptions{}
			})

			It("returns an error", func() {
				_, err := options.GetInt("Option", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
			})
		})
	})

	Describe("GetBool", func() {
		var options kubernetes.InstallationOptions
		When("option is a bool", func() {
			BeforeEach(func() {
				options = kubernetes.InstallationOptions{
					kubernetes.InstallationOption{
						Name:  "Option",
						Value: true,
						Type:  kubernetes.BooleanType,
					},
				}
			})
			It("returns a boolean value", func() {
				result, err := options.GetBool("Option", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeTrue())
			})
		})

		When("option is not a bool", func() {
			BeforeEach(func() {
				options = kubernetes.InstallationOptions{
					kubernetes.InstallationOption{
						Name:  "Option",
						Value: "aString",
						Type:  kubernetes.StringType,
					},
				}
			})
			It("panics", func() {
				Expect(func() { options.GetBool("Option", "") }).
					To(PanicWith(MatchRegexp("wrong type assertion")))
			})
		})

		When("option doesn't exist", func() {
			BeforeEach(func() {
				options = kubernetes.InstallationOptions{}
			})

			It("returns an error", func() {
				_, err := options.GetBool("Option", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
			})
		})
	})

	Describe("ForDeployment", func() {
		It("returns all options for the given deployment + shared options", func() {

		})
		It("returns no options from other deployments", func() {

		})
	})
})
