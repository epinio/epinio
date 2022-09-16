package v1_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppValidateCV Endpoint", func() {
	var (
		chartName string
		tempFile  string
		namespace string
		appName   string
		request   *http.Request
	)

	ping := func(code int, body string) {
		// request setup

		uri := fmt.Sprintf("%s%s/%s", serverURL, v1.Root,
			v1.Routes.Path("AppValidateCV", namespace, appName))
		var err error // We want `=` below to ensure that `request` is not a local variable.
		request, err = http.NewRequest("GET", uri, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		request.Header.Set("Authorization", "Bearer "+env.EpinioToken)

		// fire request, get response

		resp, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp).ToNot(BeNil())
		defer resp.Body.Close()

		// get response body

		bodyBytes, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		// check status

		Expect(resp.StatusCode).To(Equal(code), string(bodyBytes))

		// decode and check response

		if resp.StatusCode == http.StatusOK {
			r := &models.Response{}
			err = json.Unmarshal(bodyBytes, &r)
			Expect(err).ToNot(HaveOccurred())
			Expect(r.Status).To(Equal(body))
			return
		}

		r := &errors.ErrorResponse{}
		err = json.Unmarshal(bodyBytes, &r)
		Expect(err).ToNot(HaveOccurred())
		Expect(r.Errors[0].Error()).To(Equal(body))
	}

	BeforeEach(func() {
		// Appchart
		chartName = catalog.NewTmpName("chart-")
		tempFile = env.MakeAppchart(chartName)

		// Namespace
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		// Application, references new chart
		appName = catalog.NewAppName()
		out, err := env.Epinio("", "app", "create", appName, "--app-chart", chartName)
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(ContainSubstring("Ok"))
	})

	AfterEach(func() {
		env.DeleteApp(appName)
		env.DeleteNamespace(namespace)
		env.DeleteAppchart(tempFile)
	})

	It("returns ok when there are no chart values to validate", func() {
		ping(http.StatusOK, "ok")
	})

	It("returns ok for good chart values", func() {
		out, err := env.Epinio("", "app", "update", appName,
			"-v", "fake=true",
			"-v", "foo=bar",
			"-v", "bar=sna",
			"-v", "floof=3.1415926535",
			"-v", "fox=99",
			"-v", "cat=0.31415926535",
			// unknowntype, badminton, maxbad - bad spec, no good values
		)
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusOK, "ok")
	})

	It("fails for an unknown field", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "bogus=x")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "bogus": Not known`)
	})

	It("fails for an unknown field type", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "unknowntype=x")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "unknowntype": Bad spec: Unknown type "foofara"`)
	})

	It("fails for an integer field with a bad minimum", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "badminton=0")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "badminton": Bad spec: Bad minimum "hello"`)
	})

	It("fails for an integer field with a bad maximum", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "maxbad=0")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "maxbad": Bad spec: Bad maximum "world"`)
	})

	It("fails for a value out of range (< min)", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "floof=-2")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "floof": Out of bounds, "-2" too small`)
	})

	It("fails for a value out of range (> max)", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "fox=1000")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "fox": Out of bounds, "1000" too large`)
	})

	It("fails for a value out of range (not in enum)", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "bar=fox")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "bar": Illegal string "fox"`)
	})

	It("fails for a non-integer value where integer required", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "fox=hound")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "fox": Expected integer, got "hound"`)
	})

	It("fails for a non-numeric value where numeric required", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "cat=dog")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "cat": Expected number, got "dog"`)
	})

	It("fails for a non-boolean value where boolean required", func() {
		out, err := env.Epinio("", "app", "update", appName, "-v", "fake=news")
		Expect(err).ToNot(HaveOccurred(), out)

		ping(http.StatusBadRequest, `Setting "fake": Expected boolean, got "news"`)
	})
})
