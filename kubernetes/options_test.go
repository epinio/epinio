package kubernetes_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/suse/carrier/kubernetes"
)

type FakeReader struct {
	Values map[string]string
}

func (f *FakeReader) Read(opt *InstallationOption) error {
	opt.Value = f.Values[opt.Name+"-"+string(opt.DeploymentID)]
	return nil
}

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

	Describe("DynDefault", func() {
		option := InstallationOption{
			Name:         "TheName",
			DeploymentID: "SomeDeployment",
			DynDefaultFunc: func(o *InstallationOption) error {
				o.Value = "Hello"
				return nil
			},
		}
		It("calls the DynDefaultFunc", func() {
			Expect(option.DynDefault()).To(BeNil())
			Expect(option.Value).To(Equal("Hello"))
		})
	})

	Describe("SetDefault", func() {
		// The tests here are the origin for the tests of
		// `DefaultOptionsReader.Read` in files
		// `default_options_reader(_test).go`.  Any new tests
		// here should be replicated there.

		optionDynamic := InstallationOption{
			Name:         "TheName",
			DeploymentID: "SomeDeployment",
			Default:      "World",
			DynDefaultFunc: func(o *InstallationOption) error {
				o.Value = "Hello"
				return nil
			},
		}
		optionStatic := InstallationOption{
			Name:         "TheName",
			DeploymentID: "SomeDeployment",
			Default:      "World",
		}
		optionError := InstallationOption{
			Name:         "TheName",
			DeploymentID: "SomeDeployment",
			DynDefaultFunc: func(o *InstallationOption) error {
				o.Value = "Hello"
				return errors.New("an error")
			},
		}

		It("prefers the DynDefaultFunc over a static Default", func() {
			Expect(optionDynamic.SetDefault()).To(BeNil())
			Expect(optionDynamic.Value).To(Equal("Hello"))
		})

		It("uses a static Default", func() {
			Expect(optionStatic.SetDefault()).To(BeNil())
			Expect(optionStatic.Value).To(Equal("World"))
		})

		It("reports errors returned from the DynDefaultFunc", func() {
			err := optionError.SetDefault()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("an error"))
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
			Expect(len(optMap)).To(Equal(3))
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

	Describe("GetOpt", func() {
		var options InstallationOptions
		BeforeEach(func() {
			options = InstallationOptions{
				InstallationOption{
					Name:         "Global Option",
					Value:        "the value",
					Type:         StringType,
					DeploymentID: "",
				},
				InstallationOption{
					Name:         "Private Option",
					Value:        "the value",
					Type:         StringType,
					DeploymentID: "Private",
				},
				InstallationOption{
					Name:         "Option",
					Value:        "the value",
					Type:         StringType,
					DeploymentID: "Another",
				},
				InstallationOption{
					Name:         "Option",
					Value:        "the value",
					Type:         StringType,
					DeploymentID: "",
				},
			}
		})
		When("deploymentID is empty", func() {
			It("misses a wholly unknown option", func() {
				_, err := options.GetOpt("Bogus Option", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
			})
			It("misses a private option", func() {
				_, err := options.GetOpt("Private Option", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
			})
			It("finds a global-only option", func() {
				result, err := options.GetOpt("Global Option", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Name).To(Equal("Global Option"))
				Expect(result.DeploymentID).To(Equal(""))
			})
		})
		When("deploymentID is not empty", func() {
			It("finds a private-only option in its deployment", func() {
				result, err := options.GetOpt("Private Option", "Private")
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Name).To(Equal("Private Option"))
				Expect(result.DeploymentID).To(Equal("Private"))
			})
			It("finds a global-only option regardless of deployment", func() {
				result, err := options.GetOpt("Global Option", "Another")
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Name).To(Equal("Global Option"))
				Expect(result.DeploymentID).To(Equal(""))
			})
			It("finds a private option before a global option of the same name", func() {
				result, err := options.GetOpt("Option", "Another")
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Name).To(Equal("Option"))
				Expect(result.DeploymentID).To(Equal("Another"))
			})
			It("misses a private-only option outside its deployment", func() {
				_, err := options.GetOpt("Private Option", "Another")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
			})
			It("misses a wholly unknown option", func() {
				_, err := options.GetOpt("Bogus Option", "Bogus")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not set"))
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
			result := options.ForDeployment("Deployment2")
			Expect(result).ToNot(ContainElement(InstallationOption{
				Name:         "Option1",
				Value:        "SomeValue",
				DeploymentID: "Deployment1",
			}))
		})
	})

	Describe("Populate", func() {
		var options *InstallationOptions
		BeforeEach(func() {
			options = &InstallationOptions{
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
				{
					Name:         "PrivateOption2",
					DeploymentID: "Deployment2",
					Value:        "", // to be filled
				},
			}
		})

		It("returns a all options for all deployments", func() {
			fakereader := &FakeReader{
				Values: map[string]string{
					"SharedOption-":              "something-returned-by-user",
					"PrivateOption2-Deployment2": "something-returned-by-user-private2",
				},
			}
			var err error
			options, err = options.Populate(fakereader)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(*options)).To(Equal(3))
			Expect(*options).To(ContainElement(InstallationOption{
				Name:  "SharedOption",
				Value: "something-returned-by-user",
			}))
			Expect(*options).To(ContainElement(InstallationOption{
				Name:         "PrivateOption2",
				DeploymentID: "Deployment2",
				Value:        "something-returned-by-user-private2",
			}))
		})
	})
})
