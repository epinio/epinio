// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helm

import (
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	helmrelease "helm.sh/helm/v3/pkg/release"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidateField()", func() {

	It("is ok for unconstrained integer", func() {
		val, err := ValidateField("field", "1", models.ChartSetting{
			Type: "integer",
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal(int64(1)))
	})

	It("is ok for unconstrained number", func() {
		val, err := ValidateField("field", "3.1415926", models.ChartSetting{
			Type: "number",
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal(float64(3.1415926)))
	})

	It("is ok for unconstrained string", func() {
		val, err := ValidateField("field", "hallodria", models.ChartSetting{
			Type: "string",
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal("hallodria"))
	})

	It("is ok for boolean", func() {
		val, err := ValidateField("field", "true", models.ChartSetting{
			Type: "bool",
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal(true))
	})

	It("is ok for constrained integer in range", func() {
		val, err := ValidateField("field", "50", models.ChartSetting{
			Type:    "integer",
			Minimum: "0",
			Maximum: "100",
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal(int64(50)))
	})

	It("is ok for constrained number in range", func() {
		val, err := ValidateField("field", "50", models.ChartSetting{
			Type:    "number",
			Minimum: "0",
			Maximum: "100",
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal(float64(50)))
	})

	It("is ok for constrained string in enum", func() {
		val, err := ValidateField("field", "cat", models.ChartSetting{
			Type: "string",
			Enum: []string{
				"cat",
				"dog",
			},
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal("cat"))
	})

	It("is ok for unconstrained integer, enum ignored", func() {
		val, err := ValidateField("field", "100", models.ChartSetting{
			Type: "integer",
			Enum: []string{
				"cat",
				"dog",
			},
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal(int64(100)))
	})

	It("is ok for unconstrained number, enum ignored", func() {
		val, err := ValidateField("field", "100", models.ChartSetting{
			Type: "number",
			Enum: []string{
				"cat",
				"dog",
			},
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal(float64(100)))
	})

	It("is ok for unconstrained string, range ignored", func() {
		val, err := ValidateField("field", "foo", models.ChartSetting{
			Type:    "string",
			Minimum: "0",
			Maximum: "100",
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal("foo"))
	})

	It("is ok for unconstrained bool, range ignored", func() {
		val, err := ValidateField("field", "false", models.ChartSetting{
			Type:    "bool",
			Minimum: "0",
			Maximum: "100",
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal(false))
	})

	It("is ok for unconstrained bool, enum ignored", func() {
		val, err := ValidateField("field", "true", models.ChartSetting{
			Type: "bool",
			Enum: []string{
				"cat",
				"dog",
			},
		})
		Expect(err).ToNot(HaveOccurred(), val)
		Expect(val).To(Equal(true))
	})

	It("fails for an unknown field type", func() {
		_, err := ValidateField("field", "dummy", models.ChartSetting{
			Type: "foofara",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`setting "field": bad spec: unknown type "foofara"`))
	})

	It("fails for an integer field with a bad minimum", func() {
		_, err := ValidateField("field", "1", models.ChartSetting{
			Type:    "integer",
			Minimum: "hello",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`setting "field": bad spec: bad minimum "hello"`))
	})

	It("fails for an integer field with a bad maximum", func() {
		_, err := ValidateField("field", "1", models.ChartSetting{
			Type:    "integer",
			Maximum: "hello",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`setting "field": bad spec: bad maximum "hello"`))
	})

	It("fails for a value out of range (< min)", func() {
		_, err := ValidateField("field", "-2", models.ChartSetting{
			Type:    "integer",
			Minimum: "0",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`setting "field": out of bounds, "-2" too small`))
	})

	It("fails for a value out of range (> max)", func() {
		_, err := ValidateField("field", "1000", models.ChartSetting{
			Type:    "integer",
			Maximum: "100",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`setting "field": out of bounds, "1000" too large`))
	})

	It("fails for a value out of range (not in enum)", func() {
		_, err := ValidateField("field", "fox", models.ChartSetting{
			Type: "string",
			Enum: []string{
				"cat",
				"dog",
			},
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`setting "field": illegal string "fox"`))
	})

	It("fails for a non-integer value where integer required", func() {
		_, err := ValidateField("field", "hound", models.ChartSetting{
			Type: "integer",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`setting "field": expected integer, got "hound"`))
	})

	It("fails for a non-numeric value where numeric required", func() {
		_, err := ValidateField("field", "hound", models.ChartSetting{
			Type: "number",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`setting "field": expected number, got "hound"`))
	})

	It("fails for a non-boolean value where boolean required", func() {
		_, err := ValidateField("field", "hound", models.ChartSetting{
			Type: "bool",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`setting "field": expected boolean, got "hound"`))
	})
})

var _ = Describe("sameStringMap()", func() {
	It("treats nil and empty maps as equal", func() {
		Expect(sameStringMap(nil, nil)).To(BeTrue())
		Expect(sameStringMap(nil, map[string]string{})).To(BeTrue())
		Expect(sameStringMap(map[string]string{}, nil)).To(BeTrue())
	})

	It("requires exact key/value equality", func() {
		Expect(sameStringMap(
			map[string]string{"tuning": "speed"},
			map[string]string{"tuning": "speed"},
		)).To(BeTrue())
		Expect(sameStringMap(
			map[string]string{"tuning": "speed"},
			map[string]string{"tuning": "slow"},
		)).To(BeFalse())
		Expect(sameStringMap(
			map[string]string{"tuning": "speed"},
			map[string]string{},
		)).To(BeFalse())
	})

	It("does not treat a prefix key set as equal", func() {
		// Guards the same class of bug as substring chart matching:
		// overlapping names must not count as equal.
		Expect(sameStringMap(
			map[string]string{"epinio-application": "1"},
			map[string]string{"epinio-application-gateway-api": "1"},
		)).To(BeFalse())
	})
})

var _ = Describe("releaseChartConfig()", func() {
	It("returns an empty map when chartConfig is absent", func() {
		Expect(releaseChartConfig(nil)).To(Equal(map[string]string{}))
		Expect(releaseChartConfig(&helmrelease.Release{})).To(Equal(map[string]string{}))
		Expect(releaseChartConfig(&helmrelease.Release{
			Config: map[string]interface{}{},
		})).To(Equal(map[string]string{}))
	})

	It("flattens chartConfig from the release values", func() {
		rel := &helmrelease.Release{
			Config: map[string]interface{}{
				"chartConfig": map[string]interface{}{
					"tuning": "speed",
					"port":   8080,
				},
			},
		}
		Expect(releaseChartConfig(rel)).To(Equal(map[string]string{
			"tuning": "speed",
			"port":   "8080",
		}))
	})
})
