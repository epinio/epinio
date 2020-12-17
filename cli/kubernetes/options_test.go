package kubernetes_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/suse/carrier/cli/kubernetes"
)

var _ = Describe("InstallationOption", func() {
	Describe("ToOptMapKey", func() {
		option := InstallationOption{
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
		options := InstallationOptions{
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
			var sharedOption, privateOption InstallationOption
			var installationOptions InstallationOptions
			BeforeEach(func() {
				sharedOption = InstallationOption{
					Name:         "Option",
					Value:        "the old value",
					DeploymentID: "", // This is what makes it shared
				}
				privateOption = InstallationOption{
					Name:         "Option",
					Value:        "private value",
					DeploymentID: "MyDeploymentID", // This is what makes it private
				}
				installationOptions = InstallationOptions{sharedOption, privateOption}
			})
			It("returns only one instance of the shared option", func() {
				result := installationOptions.Merge(InstallationOptions{
					{Name: "Option", Value: "the new value", DeploymentID: ""},
				})
				Expect(result.GetString("Option", "")).To(Equal("the new value"))
			})

			It("doesn't overwrite private options with shared ones", func() {
				result := installationOptions.Merge(InstallationOptions{
					{Name: "Option", Value: "the new value", DeploymentID: ""},
				})
				Expect(result.GetString("Option", "MyDeploymentID")).To(Equal("private value"))
			})

			It("Returns every instance of private options (even when name match)", func() {
				result := installationOptions.Merge(InstallationOptions{
					{Name: "Option", Value: "the new value", DeploymentID: "OtherDeploymentID"},
				})
				Expect(result.GetString("Option", "MyDeploymentID")).To(Equal("private value"))
				Expect(result.GetString("Option", "OtherDeploymentID")).To(Equal("the new value"))
			})
		})
	})

	Describe("GetString", func() {
		var options InstallationOptions
		When("option is a string", func() {
			BeforeEach(func() {
				options = InstallationOptions{
					InstallationOption{
						Name:  "Option",
						Value: "the value",
						Type:  StringType,
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
				options = InstallationOptions{
					InstallationOption{
						Name:  "Option",
						Value: true,
						Type:  BooleanType,
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
				options = InstallationOptions{}
			})
			It("returns an error", func() {
				_, err := options.GetString("Option", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
			})
		})
	})

	Describe("GetInt", func() {
		var options InstallationOptions
		When("option is an int", func() {
			BeforeEach(func() {
				options = InstallationOptions{
					InstallationOption{
						Name:  "Option",
						Value: 3,
						Type:  IntType,
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
				options = InstallationOptions{
					InstallationOption{
						Name:  "Option",
						Value: true,
						Type:  BooleanType,
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
				options = InstallationOptions{}
			})

			It("returns an error", func() {
				_, err := options.GetInt("Option", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
			})
		})
	})

	Describe("GetBool", func() {
		var options InstallationOptions
		When("option is a bool", func() {
			BeforeEach(func() {
				options = InstallationOptions{
					InstallationOption{
						Name:  "Option",
						Value: true,
						Type:  BooleanType,
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
				options = InstallationOptions{
					InstallationOption{
						Name:  "Option",
						Value: "aString",
						Type:  StringType,
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
				options = InstallationOptions{}
			})

			It("returns an error", func() {
				_, err := options.GetBool("Option", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
			})
		})
	})

	Describe("ForDeployment", func() {
		var options InstallationOptions
		BeforeEach(func() {
			options = InstallationOptions{
				{
					Name:         "Option1",
					Value:        "ForDeployment1",
					DeploymentID: "Deployment1",
				},
				{
					Name:         "Option1",
					Value:        "SomeValue",
					DeploymentID: "Deployment2",
				},
				{
					Name:         "Option2",
					Value:        "SomeOtherValue",
					DeploymentID: "Deployment2",
				},
				{
					Name:         "OptionName",
					Value:        "ForAllDeployments",
					DeploymentID: "",
				},
			}
		})

		It("returns all options for the given deployment + shared options", func() {
			result := options.ForDeployment("Deployment2")
			Expect(result).To(ContainElement(InstallationOption{
				Name:         "Option1",
				Value:        "SomeValue",
				DeploymentID: "Deployment2",
			}))
			Expect(result).To(ContainElement(InstallationOption{
				Name:         "Option2",
				Value:        "SomeOtherValue",
				DeploymentID: "Deployment2",
			}))
			Expect(result).To(ContainElement(InstallationOption{
				Name:         "OptionName",
				Value:        "ForAllDeployments",
				DeploymentID: "",
			}))
			Expect(len(result)).To(Equal(3))
		})

		It("returns no options from other deployments", func() {
			result := options.ForDeployment(DeploymentID("Deployment2"))
			Expect(result).ToNot(ContainElement(InstallationOption{
				Name:         "Option1",
				Value:        "SomeValue",
				DeploymentID: "Deployment1",
			}))
		})
	})
})
