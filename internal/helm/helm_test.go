package helm

import (
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidateField()", func() {

	It("fails for an unknown field type", func() {
		_, err := ValidateField("field", "dummy", models.AppChartSetting{
			Type: "foofara",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`Setting "field": Bad spec: Unknown type "foofara"`))
	})

	It("fails for an integer field with a bad minimum", func() {
		_, err := ValidateField("field", "1", models.AppChartSetting{
			Type:    "integer",
			Minimum: "hello",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`Setting "field": Bad spec: Bad minimum "hello"`))
	})

	It("fails for an integer field with a bad maximum", func() {
		_, err := ValidateField("field", "1", models.AppChartSetting{
			Type:    "integer",
			Maximum: "hello",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`Setting "field": Bad spec: Bad maximum "hello"`))
	})

	It("fails for a value out of range (< min)", func() {
		_, err := ValidateField("field", "-2", models.AppChartSetting{
			Type:    "integer",
			Minimum: "0",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`Setting "field": Out of bounds, "-2" too small`))
	})

	It("fails for a value out of range (> max)", func() {
		_, err := ValidateField("field", "1000", models.AppChartSetting{
			Type:    "integer",
			Maximum: "100",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`Setting "field": Out of bounds, "1000" too large`))
	})

	It("fails for a value out of range (not in enum)", func() {
		_, err := ValidateField("field", "fox", models.AppChartSetting{
			Type: "string",
			Enum: []string{
				"cat",
				"dog",
			},
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`Setting "field": Illegal string "fox"`))
	})

	It("fails for a non-integer value where integer required", func() {
		_, err := ValidateField("field", "hound", models.AppChartSetting{
			Type: "integer",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`Setting "field": Expected integer, got "hound"`))
	})

	It("fails for a non-numeric value where numeric required", func() {
		_, err := ValidateField("field", "hound", models.AppChartSetting{
			Type: "number",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`Setting "field": Expected number, got "hound"`))
	})

	It("fails for a non-boolean value where boolean required", func() {
		_, err := ValidateField("field", "hound", models.AppChartSetting{
			Type: "bool",
		})
		Expect(err).To(HaveOccurred(), err.Error())
		Expect(err.Error()).To(Equal(`Setting "field": Expected boolean, got "hound"`))
	})
})
