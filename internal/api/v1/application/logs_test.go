package application_test

import (
	"net/http"

	"github.com/epinio/epinio/internal/api/v1/application"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Application Log API Endpoint unit tests", func() {
	var allowedOrigins []string
	var request *http.Request
	var theFunc func(r *http.Request) bool

	JustBeforeEach(func() {
		theFunc = application.CheckOriginFunc(allowedOrigins)
	})

	BeforeEach(func() {
		var err error
		request, err = http.NewRequest("GET", "https://somedomain.org", nil)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("CheckOriginFunc", func() {
		When("allowed origins is empty", func() {
			BeforeEach(func() {
				allowedOrigins = []string{}
				request.Header.Set("Origin", "https://somedomain.org")
			})
			It("returns true", func() {
				Expect(theFunc(request)).To(BeTrue())
			})
		})
		When("origin header is empty", func() {
			BeforeEach(func() {
				allowedOrigins = []string{"https://somedomain.org"}
				request.Header.Set("Origin", "")
			})
			It("returns true", func() {
				Expect(theFunc(request)).To(BeTrue())
			})
		})
		When("allowed origins include a '*'", func() {
			BeforeEach(func() {
				allowedOrigins = []string{"*", "https://somedomain.org"}
				request.Header.Set("Origin", "https://notthesamedomain.org")
			})
			It("returns true", func() {
				Expect(theFunc(request)).To(BeTrue())
			})
		})
		When("allowed origins match the header", func() {
			BeforeEach(func() {
				allowedOrigins = []string{"https://somedomain.org"}
				request.Header.Set("Origin", "https://somedomain.org")
			})
			It("returns true", func() {
				Expect(theFunc(request)).To(BeTrue())
			})
		})
		When("there is no match", func() {
			BeforeEach(func() {
				allowedOrigins = []string{"https://somedomain.org"}
				request.Header.Set("Origin", "https://notthesamedomain.org")
			})
			It("returns false", func() {
				Expect(theFunc(request)).To(BeFalse())
			})
		})
	})
})
