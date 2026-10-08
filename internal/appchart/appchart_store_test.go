package appchart_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/testfakes/k8sdynamic/k8sdynamicfakes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// fakeStore is a chart store recording what is done with it, in the order it is done.
type fakeStore struct {
	repository string
	stored     bool

	hasErr    error
	pushErr   error
	deleteErr error

	log *[]string
}

func (s *fakeStore) record(call string) { *s.log = append(*s.log, call) }

func (s *fakeStore) Repository() string { return s.repository }

func (s *fakeStore) HasChart(chartName, chartVersion string) (bool, error) {
	s.record("has " + chartName + ":" + chartVersion)
	return s.stored, s.hasErr
}

func (s *fakeStore) Push(archivePath string) error {
	s.record("push")
	return s.pushErr
}

func (s *fakeStore) Delete(_ context.Context, chartName, chartVersion string) error {
	s.record("delete-chart " + chartName + ":" + chartVersion)
	return s.deleteErr
}

var _ = Describe("AppChart store", func() {
	var (
		ctx    context.Context
		fakeRI *k8sdynamicfakes.FakeResourceInterface
		fakeNS *k8sdynamicfakes.FakeNamespaceableResourceInterface

		calls    []string // what happened, across the store and the cluster
		store    *fakeStore
		opened   []string // names the store was opened for
		openErr  error
		open     appchart.OpenChartStore
		notFound error

		inUse     bool
		inUseErr  error
		asked     []string // names the bound applications were asked for
		boundApps appchart.BoundApps
	)

	BeforeEach(func() {
		ctx = context.Background()
		calls = nil
		opened = nil
		openErr = nil

		inUse, inUseErr, asked = false, nil, nil
		boundApps = func(_ context.Context, name string) (bool, error) {
			asked = append(asked, name)
			return inUse, inUseErr
		}

		oldLogger := helpers.Logger
		helpers.Logger = zap.NewNop().Sugar()
		DeferCleanup(func() { helpers.Logger = oldLogger })

		notFound = k8sapierrors.NewNotFound(schema.GroupResource{Resource: "appcharts"}, "myapp")

		fakeRI = &k8sdynamicfakes.FakeResourceInterface{}
		fakeNS = &k8sdynamicfakes.FakeNamespaceableResourceInterface{}
		fakeNS.NamespaceReturns(fakeRI)

		fakeRI.CreateCalls(func(context.Context, *unstructured.Unstructured, metav1.CreateOptions, ...string) (*unstructured.Unstructured, error) {
			calls = append(calls, "create")
			return nil, nil
		})
		fakeRI.DeleteCalls(func(_ context.Context, name string, _ metav1.DeleteOptions, _ ...string) error {
			calls = append(calls, "delete-appchart "+name)
			return nil
		})

		store = &fakeStore{repository: "oci://registry.example.com/epinio-charts/myapp", log: &calls}
		open = func(_ context.Context, name string) (appchart.ChartStore, error) {
			opened = append(opened, name)
			return store, openErr
		}
	})

	saveChart := func(name, version, chartType string) string {
		path, err := chartutil.Save(&chart.Chart{
			Metadata: &chart.Metadata{APIVersion: "v2", Name: name, Version: version, Type: chartType},
		}, GinkgoT().TempDir())
		Expect(err).ToNot(HaveOccurred())
		return path
	}

	// existing makes the cluster hold an application chart, with the given chart and repository.
	existing := func(helmChart, helmRepo string) {
		fakeRI.GetReturns(&unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "myapp"},
			"spec": map[string]interface{}{
				"helmChart": helmChart,
				"helmRepo":  helmRepo,
			},
		}}, nil)
	}

	Describe("LoadChartArchive", func() {
		It("loads an application chart", func() {
			ch, err := appchart.LoadChartArchive(saveChart("mychart", "0.1.0", ""))
			Expect(err).ToNot(HaveOccurred())
			Expect(ch.Metadata.Name).To(Equal("mychart"))
			Expect(ch.Metadata.Version).To(Equal("0.1.0"))
		})

		It("accepts the explicit application type", func() {
			_, err := appchart.LoadChartArchive(saveChart("mychart", "0.1.0", "application"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects a file which is not a chart archive", func() {
			path := filepath.Join(GinkgoT().TempDir(), "notachart.tgz")
			Expect(os.WriteFile(path, []byte("hello"), 0600)).To(Succeed())

			_, err := appchart.LoadChartArchive(path)

			var invalid *appchart.InvalidChartError
			Expect(errors.As(err, &invalid)).To(BeTrue())
			Expect(invalid.Reason).To(Equal("the upload is not a valid helm chart archive"))
		})

		It("rejects a file which does not exist", func() {
			_, err := appchart.LoadChartArchive(filepath.Join(GinkgoT().TempDir(), "missing.tgz"))

			var invalid *appchart.InvalidChartError
			Expect(errors.As(err, &invalid)).To(BeTrue())
		})

		It("rejects a library chart", func() {
			_, err := appchart.LoadChartArchive(saveChart("mychart", "0.1.0", "library"))

			var invalid *appchart.InvalidChartError
			Expect(errors.As(err, &invalid)).To(BeTrue())
			Expect(invalid.Reason).To(ContainSubstring(`chart type "library" is not usable as application chart`))
		})

		It("rejects a chart which is not valid", func() {
			// chartutil.Save refuses to write such a chart. Write the archive by hand, without
			// version in Chart.yaml.
			path := filepath.Join(GinkgoT().TempDir(), "noversion.tgz")
			file, err := os.Create(filepath.Clean(path))
			Expect(err).ToNot(HaveOccurred())

			gz := gzip.NewWriter(file)
			tw := tar.NewWriter(gz)
			content := []byte("apiVersion: v2\nname: mychart\n")
			Expect(tw.WriteHeader(&tar.Header{Name: "mychart/Chart.yaml", Mode: 0600, Size: int64(len(content))})).To(Succeed())
			_, err = tw.Write(content)
			Expect(err).ToNot(HaveOccurred())
			Expect(tw.Close()).To(Succeed())
			Expect(gz.Close()).To(Succeed())
			Expect(file.Close()).To(Succeed())

			_, err = appchart.LoadChartArchive(path)

			// helm already refuses it while loading the archive.
			var invalid *appchart.InvalidChartError
			Expect(errors.As(err, &invalid)).To(BeTrue())
			Expect(invalid.Details).To(ContainSubstring("version"))
		})
	})

	Describe("Push", func() {
		var request appchart.PushRequest

		BeforeEach(func() {
			fakeRI.GetReturns(nil, notFound)

			path := saveChart("mychart", "0.1.0", "")
			ch, err := appchart.LoadChartArchive(path)
			Expect(err).ToNot(HaveOccurred())

			request = appchart.PushRequest{
				Name:             "myapp",
				Description:      "long",
				ShortDescription: "short",
				ArchivePath:      path,
				Chart:            ch,
			}
		})

		It("creates the application chart before it stores the chart", func() {
			result, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).ToNot(HaveOccurred())

			Expect(result).To(Equal(&models.AppChartPushResponse{
				Name:      "myapp",
				HelmChart: "mychart:0.1.0",
				HelmRepo:  "oci://registry.example.com/epinio-charts/myapp",
			}))
			Expect(opened).To(Equal([]string{"myapp"}))
			Expect(calls).To(Equal([]string{"create", "has mychart:0.1.0", "push"}))
		})

		It("creates the application chart for the stored chart", func() {
			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).ToNot(HaveOccurred())

			_, created, _, _ := fakeRI.CreateArgsForCall(0)
			Expect(created.GetName()).To(Equal("myapp"))

			spec, _, err := unstructured.NestedMap(created.Object, "spec")
			Expect(err).ToNot(HaveOccurred())
			Expect(spec).To(HaveKeyWithValue("helmChart", "mychart:0.1.0"))
			Expect(spec).To(HaveKeyWithValue("helmRepo", "oci://registry.example.com/epinio-charts/myapp"))
			Expect(spec).To(HaveKeyWithValue("description", "long"))
			Expect(spec).To(HaveKeyWithValue("shortDescription", "short"))
		})

		It("refuses a name which is taken, without touching the store", func() {
			existing("https://example.com/chart.tgz", "")

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)

			var exists *appchart.AlreadyExistsError
			Expect(errors.As(err, &exists)).To(BeTrue())
			Expect(exists.Name).To(Equal("myapp"))
			Expect(opened).To(BeEmpty())
			Expect(calls).To(BeEmpty())
		})

		It("reports a failure to look for the application chart", func() {
			fakeRI.GetReturns(nil, errors.New("api server unavailable"))

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).To(MatchError(ContainSubstring("api server unavailable")))
			Expect(opened).To(BeEmpty())
		})

		It("creates nothing when the store cannot be opened", func() {
			openErr = errors.New("no registry")

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).To(MatchError(ContainSubstring("no registry")))
			Expect(calls).To(BeEmpty())
		})

		It("reports a name taken by a concurrent push as taken", func() {
			fakeRI.CreateCalls(func(context.Context, *unstructured.Unstructured, metav1.CreateOptions, ...string) (*unstructured.Unstructured, error) {
				calls = append(calls, "create")
				return nil, k8sapierrors.NewAlreadyExists(schema.GroupResource{Resource: "appcharts"}, "myapp")
			})

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)

			var exists *appchart.AlreadyExistsError
			Expect(errors.As(err, &exists)).To(BeTrue())
			Expect(exists.Name).To(Equal("myapp"))
			Expect(calls).To(Equal([]string{"create"}))
		})

		It("replaces a chart left behind in the registry, once the name is reserved", func() {
			store.stored = true

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(calls).To(Equal([]string{"create", "has mychart:0.1.0", "delete-chart mychart:0.1.0", "push"}))
		})

		It("removes the application chart again when the chart left behind cannot be removed", func() {
			store.stored = true
			store.deleteErr = errors.New("registry down")

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).To(MatchError(ContainSubstring("registry down")))
			Expect(calls).To(Equal([]string{
				"create", "has mychart:0.1.0", "delete-chart mychart:0.1.0", "delete-appchart myapp",
			}))
		})

		It("removes the application chart again when it cannot tell whether the chart is stored", func() {
			store.hasErr = errors.New("registry down")

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).To(MatchError(ContainSubstring("registry down")))
			Expect(calls).To(Equal([]string{"create", "has mychart:0.1.0", "delete-appchart myapp"}))
		})

		It("does not store the chart when the application chart cannot be created", func() {
			fakeRI.CreateCalls(func(context.Context, *unstructured.Unstructured, metav1.CreateOptions, ...string) (*unstructured.Unstructured, error) {
				calls = append(calls, "create")
				return nil, errors.New("create denied")
			})

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).To(MatchError(ContainSubstring("create denied")))
			Expect(calls).To(Equal([]string{"create"}))
		})

		It("removes the application chart again when the push fails", func() {
			store.pushErr = errors.New("registry rejected the chart")

			result, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("registry rejected the chart")))
			Expect(err).To(MatchError(ContainSubstring("pushing the chart to the registry")))
			Expect(calls).To(Equal([]string{"create", "has mychart:0.1.0", "push", "delete-appchart myapp"}))
		})

		It("still reports the failed push when the application chart cannot be removed", func() {
			store.pushErr = errors.New("registry rejected the chart")
			fakeRI.DeleteReturns(errors.New("delete denied"))
			fakeRI.DeleteCalls(nil)

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).To(MatchError(ContainSubstring("registry rejected the chart")))
			Expect(fakeRI.DeleteCallCount()).To(Equal(1))
		})

		It("removes the application chart with a context which is not canceled", func() {
			store.pushErr = errors.New("registry rejected the chart")
			canceled, cancel := context.WithCancel(ctx)
			cancel()

			// The existence check, and the creation, do not look at the context here.
			_, err := appchart.Push(canceled, fakeNS, open, boundApps, request)
			Expect(err).To(HaveOccurred())

			deleteCtx, _, _, _ := fakeRI.DeleteArgsForCall(0)
			Expect(deleteCtx.Err()).ToNot(HaveOccurred())
		})
	})

	Describe("Push, to the name of an existing application chart", func() {
		const storedRepo = "oci://registry.example.com/epinio-charts/myapp"

		var (
			request appchart.PushRequest
			patched map[string]interface{} // spec the application chart was updated to
		)

		BeforeEach(func() {
			patched = nil
			fakeRI.UpdateCalls(func(_ context.Context, obj *unstructured.Unstructured, _ metav1.UpdateOptions, _ ...string) (*unstructured.Unstructured, error) {
				calls = append(calls, "update")
				patched, _, _ = unstructured.NestedMap(obj.Object, "spec")
				return obj, nil
			})

			path := saveChart("mychart", "0.2.0", "")
			ch, err := appchart.LoadChartArchive(path)
			Expect(err).ToNot(HaveOccurred())

			request = appchart.PushRequest{Name: "myapp", Description: "long", ArchivePath: path, Chart: ch}
		})

		It("replaces the chart of a pushed application chart, storing it before the change", func() {
			existing("mychart:0.1.0", storedRepo)

			result, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).ToNot(HaveOccurred())

			Expect(result).To(Equal(&models.AppChartPushResponse{
				Name: "myapp", HelmChart: "mychart:0.2.0", HelmRepo: storedRepo,
			}))
			Expect(asked).To(Equal([]string{"myapp"}))
			Expect(calls).To(Equal([]string{"push", "update"}))
			Expect(patched).To(HaveKeyWithValue("helmChart", "mychart:0.2.0"))
			Expect(patched).To(HaveKeyWithValue("helmRepo", storedRepo))
			Expect(patched).To(HaveKeyWithValue("description", "long"))
		})

		It("removes the replaced chart last, when the new chart has another name", func() {
			existing("oldchart:0.1.0", storedRepo)

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(calls).To(Equal([]string{"push", "update", "delete-chart oldchart:0.1.0"}))
		})

		It("succeeds when the replaced chart cannot be removed", func() {
			existing("oldchart:0.1.0", storedRepo)
			store.deleteErr = errors.New("registry down")

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).ToNot(HaveOccurred())
		})

		It("refuses while applications use the application chart, and stores nothing", func() {
			existing("mychart:0.1.0", storedRepo)
			inUse = true

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)

			var inUseError *appchart.InUseError
			Expect(errors.As(err, &inUseError)).To(BeTrue())
			Expect(calls).To(BeEmpty())
		})

		It("stores nothing when it cannot tell whether applications use the application chart", func() {
			existing("mychart:0.1.0", storedRepo)
			inUseErr = errors.New("api server unavailable")

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).To(MatchError(ContainSubstring("api server unavailable")))
			Expect(calls).To(BeEmpty())
		})

		It("reports the name of a chart in another registry, with the same path, as taken", func() {
			existing("mychart:0.1.0", "oci://elsewhere.example.com/epinio-charts/myapp")

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)

			var exists *appchart.AlreadyExistsError
			Expect(errors.As(err, &exists)).To(BeTrue())
			Expect(asked).To(BeEmpty())
			Expect(calls).To(BeEmpty())
		})

		It("leaves the application chart alone when the chart cannot be stored", func() {
			existing("mychart:0.1.0", storedRepo)
			store.pushErr = errors.New("registry rejected the chart")

			_, err := appchart.Push(ctx, fakeNS, open, boundApps, request)
			Expect(err).To(MatchError(ContainSubstring("registry rejected the chart")))
			Expect(calls).To(Equal([]string{"push"}))
		})
	})

	Describe("UpdateWithChart", func() {
		const storedRepo = "oci://registry.example.com/epinio-charts/myapp"

		var updated bool

		BeforeEach(func() {
			updated = false
			fakeRI.UpdateCalls(func(_ context.Context, obj *unstructured.Unstructured, _ metav1.UpdateOptions, _ ...string) (*unstructured.Unstructured, error) {
				updated = true
				return obj, nil
			})
		})

		locked := func(err error) bool {
			var lockedErr *appchart.LocationLockedError
			return errors.As(err, &lockedErr)
		}

		It("refuses to change the location of a chart which applications use", func() {
			existing("https://example.com/chart-1.tgz", "")
			inUse = true

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{HelmChart: "https://example.com/chart-2.tgz"})

			var inUseError *appchart.InUseError
			Expect(errors.As(err, &inUseError)).To(BeTrue())
			Expect(asked).To(Equal([]string{"myapp"}))
			Expect(updated).To(BeFalse())
		})

		It("reports a stored chart which applications use as in use, without opening the store", func() {
			existing("mychart:0.1.0", storedRepo)
			inUse = true

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{HelmChart: "other:1.0.0"})

			var inUseError *appchart.InUseError
			Expect(errors.As(err, &inUseError)).To(BeTrue())
			Expect(opened).To(BeEmpty())
			Expect(updated).To(BeFalse())
		})

		It("updates the other fields of a chart which applications use, without looking for them", func() {
			existing("https://example.com/chart-1.tgz", "")
			inUse = true

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{
				Description: "new", HelmChart: "https://example.com/chart-1.tgz",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(asked).To(BeEmpty())
			Expect(updated).To(BeTrue())
		})

		It("changes nothing when it cannot tell whether applications use the chart", func() {
			existing("https://example.com/chart-1.tgz", "")
			inUseErr = errors.New("api server unavailable")

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{HelmChart: "https://example.com/chart-2.tgz"})
			Expect(err).To(MatchError(ContainSubstring("api server unavailable")))
			Expect(updated).To(BeFalse())
		})

		It("refuses to change the chart of a stored chart", func() {
			existing("mychart:0.1.0", storedRepo)

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{HelmChart: "other:1.0.0"})
			Expect(locked(err)).To(BeTrue())
			Expect(updated).To(BeFalse())
		})

		It("refuses to change the repository of a stored chart", func() {
			existing("mychart:0.1.0", storedRepo)

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{HelmRepo: "https://example.com/charts"})
			Expect(locked(err)).To(BeTrue())
			Expect(updated).To(BeFalse())
		})

		It("accepts the unchanged location of a stored chart", func() {
			existing("mychart:0.1.0", storedRepo)

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{
				Description: "new", HelmChart: "mychart:0.1.0", HelmRepo: storedRepo,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(opened).To(BeEmpty())
			Expect(updated).To(BeTrue())
		})

		It("updates the other fields of a stored chart without opening the store", func() {
			existing("mychart:0.1.0", storedRepo)

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{Description: "new"})
			Expect(err).ToNot(HaveOccurred())
			Expect(opened).To(BeEmpty())
			Expect(updated).To(BeTrue())
		})

		It("changes the location of a chart given by url, without opening the store", func() {
			existing("https://example.com/chart-1.tgz", "")

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{HelmChart: "https://example.com/chart-2.tgz"})
			Expect(err).ToNot(HaveOccurred())
			Expect(opened).To(BeEmpty())
			Expect(updated).To(BeTrue())
		})

		It("changes the location of a chart in another registry, with the same path", func() {
			existing("mychart:0.1.0", "oci://elsewhere.example.com/epinio-charts/myapp")

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{HelmChart: "mychart:0.2.0"})
			Expect(err).ToNot(HaveOccurred())
			Expect(opened).To(Equal([]string{"myapp"}))
			Expect(updated).To(BeTrue())
		})

		It("changes nothing when the store cannot be opened", func() {
			existing("mychart:0.1.0", storedRepo)
			openErr = errors.New("no registry")

			err := appchart.UpdateWithChart(ctx, fakeNS, open, boundApps, "myapp", models.AppChartUpdateRequest{HelmChart: "other:1.0.0"})
			Expect(err).To(MatchError(ContainSubstring("no registry")))
			Expect(updated).To(BeFalse())
		})
	})

	Describe("DeleteWithChart", func() {
		const storedRepo = "oci://registry.example.com/epinio-charts/myapp"

		It("removes the stored chart from the registry before it deletes the application chart", func() {
			existing("mychart:0.1.0", storedRepo)

			Expect(appchart.DeleteWithChart(ctx, fakeNS, open, "myapp")).To(Succeed())

			Expect(opened).To(Equal([]string{"myapp"}))
			Expect(calls).To(Equal([]string{"delete-chart mychart:0.1.0", "delete-appchart myapp"}))
		})

		It("keeps the application chart when the chart cannot be removed", func() {
			existing("mychart:0.1.0", storedRepo)
			store.deleteErr = errors.New("registry down")

			err := appchart.DeleteWithChart(ctx, fakeNS, open, "myapp")
			Expect(err).To(MatchError(ContainSubstring("removing the chart from the registry")))
			Expect(err).To(MatchError(ContainSubstring("registry down")))
			Expect(calls).To(Equal([]string{"delete-chart mychart:0.1.0"}))
		})

		It("keeps the application chart when the store cannot be opened", func() {
			existing("mychart:0.1.0", storedRepo)
			openErr = errors.New("no registry")

			err := appchart.DeleteWithChart(ctx, fakeNS, open, "myapp")
			Expect(err).To(MatchError(ContainSubstring("no registry")))
			Expect(calls).To(BeEmpty())
		})

		It("leaves the registry alone for a chart in another registry, with the same path", func() {
			existing("mychart:0.1.0", "oci://other.example.com/epinio-charts/myapp")

			Expect(appchart.DeleteWithChart(ctx, fakeNS, open, "myapp")).To(Succeed())

			Expect(opened).To(Equal([]string{"myapp"}))
			Expect(calls).To(Equal([]string{"delete-appchart myapp"}))
		})

		DescribeTable("leaves the registry alone for charts which were not pushed",
			func(helmChart, helmRepo string) {
				existing(helmChart, helmRepo)

				Expect(appchart.DeleteWithChart(ctx, fakeNS, open, "myapp")).To(Succeed())

				Expect(opened).To(BeEmpty())
				Expect(calls).To(Equal([]string{"delete-appchart myapp"}))
			},
			Entry("url", "https://example.com/chart.tgz", ""),
			Entry("classic helm repository", "mychart:0.1.0", "https://charts.example.com"),
			Entry("oci chart elsewhere", "mychart:0.1.0", "oci://ghcr.io/epinio/charts"),
			Entry("oci chart in the path of another application chart", "mychart:0.1.0",
				"oci://registry.example.com/epinio-charts/other"),
		)

		It("deletes an application chart which is already gone from the cluster", func() {
			fakeRI.GetReturns(nil, notFound)
			fakeRI.DeleteCalls(nil)
			fakeRI.DeleteReturns(notFound)

			err := appchart.DeleteWithChart(ctx, fakeNS, open, "myapp")
			Expect(k8sapierrors.IsNotFound(err)).To(BeTrue())
			Expect(opened).To(BeEmpty())
		})

		It("reports a failure to look for the application chart", func() {
			fakeRI.GetReturns(nil, errors.New("api server unavailable"))

			err := appchart.DeleteWithChart(ctx, fakeNS, open, "myapp")
			Expect(err).To(MatchError(ContainSubstring("api server unavailable")))
			Expect(calls).To(BeEmpty())
		})
	})
})
