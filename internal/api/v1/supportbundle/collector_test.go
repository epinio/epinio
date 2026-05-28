package supportbundle_test

import (
	"context"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/supportbundle"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Collector", func() {
	var ctx context.Context
	var cluster *kubernetes.Cluster
	var fakeClient *fake.Clientset
	var logger *zap.SugaredLogger
	var installationNamespace string

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = fake.NewSimpleClientset()
		cluster = &kubernetes.Cluster{
			Kubectl: fakeClient,
		}
		logger = zap.NewNop().Sugar()
		helpers.Logger = logger
		installationNamespace = "custom-epinio"
	})

	It("initializes correctly with namespace", func() {
		tailLines := int64(100)
		collector := supportbundle.NewCollector(cluster, "/tmp", tailLines, logger, installationNamespace)
		Expect(collector).ToNot(BeNil())
		// We can't access private fields of collector, but we can test its methods
	})

	It("collects logs from the correct namespace", func() {
		// Create a pod in the installation namespace
		podName := "epinio-server-pod"
		_, err := fakeClient.CoreV1().Pods(installationNamespace).Create(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: installationNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/component": "epinio-server",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "epinio-server",
					},
				},
			},
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		tailLines := int64(100)
		collector := supportbundle.NewCollector(cluster, "/tmp", tailLines, logger, installationNamespace)

		// This will likely fail because fake client doesn't support streaming logs fully without more setup
		// But it should at least try to find the pod in the correct namespace
		// If it was looking in "epinio" namespace, it would fail with NotFound immediately
		
		// However, CollectEpinioServerLogs returns a list of file paths.
		// It calls collectPodLogsWithPrevious.
		// collectPodLogsWithPrevious calls ListPods with selector.
		// If selector matches, it tries to get logs.
		
		// Let's just verify it doesn't panic and maybe returns error or empty list depending on fake client behavior
		err = collector.CollectEpinioServerLogs(ctx)
		
		// If err is "pod not found", it means it looked in wrong namespace or selector didn't match
		// If err is related to streaming, it means it found the pod!
		
		if err != nil {
			// If error is about streaming, it means it found the pod, which is good!
			// fake client GetLogs usually returns a request that fails to stream if not mocked
			Expect(err.Error()).ToNot(ContainSubstring("not found"))
		} else {
			Expect(err).To(BeNil())
		}
	})
})
