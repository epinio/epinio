package namespace_test

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	apinamespace "github.com/epinio/epinio/internal/api/v1/namespace"
	"github.com/epinio/epinio/internal/api/v1/namespace/namespacefakes"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/stdr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespace Controller", func() {
	var c *gin.Context
	var w *httptest.ResponseRecorder
	var controller *apinamespace.Controller

	var fakeNamespaceService *namespacefakes.FakeNamespaceService
	var fakeAuthService *namespacefakes.FakeAuthService

	BeforeEach(func() {
		rand.Seed(time.Now().UnixNano())
		fakeNamespaceService = &namespacefakes.FakeNamespaceService{}
		fakeAuthService = &namespacefakes.FakeAuthService{}
		controller = apinamespace.NewController(fakeNamespaceService, fakeAuthService)

		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
	})

	setupRequestWithBody := func(body string) {
		var bodyReader io.Reader
		if body != "" {
			bodyReader = strings.NewReader(body)
		}

		req, err := http.NewRequest(http.MethodGet, "http://url.com/endpoint", bodyReader)
		Expect(err).ToNot(HaveOccurred())

		ctx := requestctx.WithLogger(context.Background(), stdr.New(nil))
		c.Request = req.Clone(ctx)
	}

	Context("Create namespace", func() {

		When("empty body is provided", func() {
			It("returns status code 400", func() {
				setupRequestWithBody("")

				apiErr := controller.Create(c)
				Expect(apiErr).To(HaveOccurred())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusBadRequest))
			})
		})

		When("empty JSON body is provided", func() {
			It("returns status code 400", func() {
				setupRequestWithBody(`{}`)

				apiErr := controller.Create(c)
				Expect(apiErr).To(HaveOccurred())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusBadRequest))
			})
		})

		When("JSON body with empty name or null is provided", func() {
			It("returns status code 400", func() {
				setupRequestWithBody(`{"name":null}`)

				apiErr := controller.Create(c)
				Expect(apiErr).To(HaveOccurred())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusBadRequest))

				setupRequestWithBody(`{"name":""}`)

				apiErr = controller.Create(c)
				Expect(apiErr).To(HaveOccurred())
				Expect(apiErr.FirstStatus()).To(Equal(http.StatusBadRequest))
			})
		})

		When("valid body is provided", func() {

			BeforeEach(func() {
				setupRequestWithBody(`{"name":"namespace-name"}`)
			})

			When("namespace exists returns an error", func() {
				It("returns status code 500", func() {
					fakeNamespaceService.ExistsStub = func(ctx context.Context, namespace string) (bool, error) {
						return false, errors.New("something bad happened")
					}

					apiErr := controller.Create(c)
					Expect(apiErr).To(HaveOccurred())
					Expect(apiErr.FirstStatus()).To(Equal(http.StatusInternalServerError))
				})
			})

			When("namespace already exists", func() {
				It("returns status code 409", func() {
					fakeNamespaceService.ExistsStub = func(ctx context.Context, namespace string) (bool, error) {
						return true, nil
					}

					apiErr := controller.Create(c)
					Expect(apiErr).To(HaveOccurred())
					Expect(apiErr.FirstStatus()).To(Equal(http.StatusConflict))
				})
			})

			When("namespace doesn't exists", func() {

				BeforeEach(func() {
					fakeNamespaceService.ExistsStub = func(ctx context.Context, namespace string) (bool, error) {
						return false, nil
					}
				})

				When("auth service errored", func() {
					It("returns status code 409", func() {
						fakeAuthService.AddNamespaceToUserStub = func(ctx context.Context, namespace string, userID string) error {
							return errors.New("something bad happened")
						}

						apiErr := controller.Create(c)
						Expect(apiErr).To(HaveOccurred())
						Expect(apiErr.FirstStatus()).To(Equal(http.StatusInternalServerError))
					})
				})

				When("auth service is successful", func() {

					BeforeEach(func() {
						fakeAuthService.AddNamespaceToUserStub = func(ctx context.Context, namespace string, userID string) error {
							return nil
						}
					})

					When("create fails", func() {

						It("returns status code 500", func() {
							fakeNamespaceService.CreateStub = func(ctx context.Context, namespace string) error {
								return errors.New("failing create")
							}

							apiErr := controller.Create(c)
							Expect(apiErr).To(HaveOccurred())
							Expect(apiErr.FirstStatus()).To(Equal(http.StatusInternalServerError))
						})
					})

					When("create succeed", func() {

						It("returns status code 201", func() {
							fakeNamespaceService.CreateStub = func(ctx context.Context, namespace string) error {
								return nil
							}

							apiErr := controller.Create(c)
							Expect(apiErr).ToNot(HaveOccurred())
							Expect(w.Result().StatusCode).To(Equal(http.StatusCreated))
						})
					})
				})
			})
		})
	})
})
