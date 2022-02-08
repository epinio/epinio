package application

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/utils/pointer"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/epinio/epinio/helpers/cahash"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/internal/s3manager"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

const (
	AWSCLIImage = "amazon/aws-cli:2.0.52"
	BashImage   = "bash"
)

type stageParam struct {
	models.AppRef
	BlobUID             string
	BuilderImage        string
	Environment         models.EnvVariableList
	Owner               metav1.OwnerReference
	RegistryURL         string
	S3ConnectionDetails s3manager.ConnectionDetails
	Stage               models.StageRef
	Username            string
	PreviousStageID     string
	RegistryCASecret    string
	RegistryCAHash      string
}

// ImageURL returns the URL of the container image to be, using the
// ImageID. The ImageURL is later used in app.yml and to send in the
// stage response.
func (app *stageParam) ImageURL(registryURL string) string {
	return fmt.Sprintf("%s/%s-%s:%s", registryURL, app.Namespace, app.Name, app.Stage.ID)
}

// ensurePVC creates a PVC for the application if one doesn't already exist.
// This PVC is used to store the application source blobs (as they are uploaded
// on the "upload" endpoint). It is also mounted in the staging pod, as the
// "source" workspace.
// The same PVC stores the application's build cache (on a separate directory).
func ensurePVC(ctx context.Context, cluster *kubernetes.Cluster, ar models.AppRef) error {
	_, err := cluster.Kubectl.CoreV1().PersistentVolumeClaims(helmchart.StagingNamespace).
		Get(ctx, ar.MakePVCName(), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) { // Unknown error, irrelevant to non-existence
		return err
	}
	if err == nil { // pvc already exists
		return nil
	}

	// From here on, only if the PVC is missing
	_, err = cluster.Kubectl.CoreV1().PersistentVolumeClaims(helmchart.StagingNamespace).
		Create(ctx, &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ar.MakePVCName(),
				Namespace: helmchart.StagingNamespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}, metav1.CreateOptions{})

	return err
}

// Stage handles the API endpoint /namespaces/:namespace/applications/:app/stage
// It creates a Job resource to stage the app
func (hc Controller) Stage(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := tracelog.Logger(ctx)

	namespace := c.Param("namespace")
	name := c.Param("app")
	username := requestctx.User(ctx)

	req := models.StageRequest{}
	if err := c.BindJSON(&req); err != nil {
		return apierror.NewBadRequest("Failed to unmarshal app stage request", err.Error())
	}
	if name != req.App.Name {
		return apierror.NewBadRequest("name parameter from URL does not match name param in body")
	}
	if namespace != req.App.Namespace {
		return apierror.NewBadRequest("namespace parameter from URL does not match namespace param in body")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	// check application resource
	app, err := application.Get(ctx, cluster, req.App)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierror.AppIsNotKnown("cannot stage app, application resource is missing")
		}
		return apierror.InternalError(err, "failed to get the application resource")
	}

	builderImage, builderErr := getBuilderImage(req, app)
	if builderErr != nil {
		return builderErr
	}

	log.Info("staging app", "namespace", namespace, "app", req)

	staging, err := application.CurrentlyStaging(ctx, cluster, req.App.Namespace, req.App.Name)
	if err != nil {
		return apierror.InternalError(err)
	}
	if staging {
		return apierror.NewBadRequest("Staging job for image ID still running")
	}

	s3ConnectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster,
		helmchart.StagingNamespace, helmchart.S3ConnectionDetailsSecretName)
	if err != nil {
		return apierror.InternalError(err, "failed to fetch the S3 connection details")
	}

	blobUID, blobErr := getBlobUID(ctx, s3ConnectionDetails, req, app)
	if blobErr != nil {
		return blobErr
	}

	// Create uid identifying the staging job to be

	uid, err := randstr.Hex16()
	if err != nil {
		return apierror.InternalError(err, "failed to generate a uid")
	}

	environment, err := application.Environment(ctx, cluster, req.App)
	if err != nil {
		return apierror.InternalError(err, "failed to access application runtime environment")
	}

	owner := metav1.OwnerReference{
		APIVersion: app.GetAPIVersion(),
		Kind:       app.GetKind(),
		Name:       app.GetName(),
		UID:        app.GetUID(),
	}

	// Determine stage id of currently running deployment, fallback to itself when no such exists.
	// From the view of the new build we are about to create this is the previous id.
	previousID, err := application.StageID(ctx, cluster, req.App)
	if err != nil {
		return apierror.InternalError(err, "failed to determine active application stage id")
	}
	if previousID == "" {
		previousID = uid
	}

	registryPublicURL, err := getRegistryURL(ctx, cluster)
	if err != nil {
		return apierror.InternalError(err, "getting the Epinio registry public URL")
	}

	registryCertificateSecret := viper.GetString("registry-certificate-secret")
	registryCertificateHash := ""
	if registryCertificateSecret != "" {
		registryCertificateHash, err = getRegistryCertificateHash(ctx, cluster, helmchart.StagingNamespace, registryCertificateSecret)
		if err != nil {
			return apierror.InternalError(err, "cannot calculate Certificate hash")
		}
	}

	params := stageParam{
		AppRef:              req.App,
		BuilderImage:        builderImage,
		BlobUID:             blobUID,
		Environment:         environment.List(),
		Owner:               owner,
		RegistryURL:         registryPublicURL,
		S3ConnectionDetails: s3ConnectionDetails,
		Stage:               models.NewStage(uid),
		PreviousStageID:     previousID,
		Username:            username,
		RegistryCAHash:      registryCertificateHash,
		RegistryCASecret:    registryCertificateSecret,
	}

	err = ensurePVC(ctx, cluster, req.App)
	if err != nil {
		return apierror.InternalError(err, "failed to ensure a PersistenVolumeClaim for the application source and cache")
	}

	job, jobenv := newJobRun(params)

	// Note: The secret is deleted with the job in function `Unstage()`.
	err = cluster.CreateSecret(ctx, helmchart.StagingNamespace, *jobenv)
	if err != nil {
		return apierror.InternalError(err, fmt.Sprintf("failed to create job env: %#v", jobenv))
	}

	err = cluster.CreateJob(ctx, helmchart.StagingNamespace, job)
	if err != nil {
		return apierror.InternalError(err, fmt.Sprintf("failed to create job run: %#v", job))
	}

	if err := updateApp(ctx, cluster, app, params); err != nil {
		return apierror.InternalError(err, "updating application CR with staging information")
	}

	imageURL := params.ImageURL(params.RegistryURL)

	log.Info("staged app", "namespace", helmchart.StagingNamespace, "app", params.AppRef, "uid", uid, "image", imageURL)

	response.OKReturn(c, models.StageResponse{
		Stage:    models.NewStage(uid),
		ImageURL: imageURL,
	})
	return nil
}

// Staged handles the API endpoint /namespaces/:namespace/staging/:stage_id/complete
// It waits for the Job resource staging the app to complete
func (hc Controller) Staged(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	namespace := c.Param("namespace")
	id := c.Param("stage_id")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	if err := hc.validateNamespace(ctx, cluster, namespace); err != nil {
		return err
	}

	// Wait for the staging to be done, then check if it ended in failure.
	// Select the job for this stage `id`.
	selector := fmt.Sprintf("app.kubernetes.io/component=staging,app.kubernetes.io/part-of=%s,epinio.suse.org/stage-id=%s",
		namespace, id)

	jobList, err := cluster.ListJobs(ctx, helmchart.StagingNamespace, selector)
	if err != nil {
		return apierror.InternalError(err)
	}
	if len(jobList.Items) == 0 {
		return apierror.InternalError(fmt.Errorf("no jobs in %s with selector %s", namespace, selector))
	}

	for _, job := range jobList.Items {
		// Wait for job to be done
		err = cluster.WaitForJobDone(ctx, helmchart.StagingNamespace, job.Name, duration.ToAppBuilt())
		if err != nil {
			return apierror.InternalError(err)
		}
		// Check job for failure
		failed, err := cluster.IsJobFailed(ctx, job.Name, helmchart.StagingNamespace)
		if err != nil {
			return apierror.InternalError(err)
		}
		if failed {
			return apierror.NewInternalError("Failed to stage",
				fmt.Sprintf("stage-id = %s", id))
		}
	}

	response.OK(c)
	return nil
}

func validateBlob(ctx context.Context, blobUID string, app models.AppRef, s3ConnectionDetails s3manager.ConnectionDetails) apierror.APIErrors {

	manager, err := s3manager.New(s3ConnectionDetails)
	if err != nil {
		return apierror.InternalError(err, "creating an S3 manager")
	}

	blobMeta, err := manager.Meta(ctx, blobUID)
	if err != nil {
		return apierror.InternalError(err, "querying blob id meta-data")
	}

	blobApp, ok := blobMeta["App"]
	if !ok {
		return apierror.NewInternalError("blob has no app name meta data")
	}
	if blobApp != app.Name {
		return apierror.NewBadRequest(
			"blob app mismatch",
			"expected: "+app.Name,
			"found: "+blobApp)
	}

	blobNamespace, ok := blobMeta["Namespace"]
	if !ok {
		return apierror.NewInternalError("blob has no namespace meta data")
	}
	if blobNamespace != app.Namespace {
		return apierror.NewBadRequest(
			"blob namespace mismatch",
			"expected: "+app.Namespace,
			"found: "+blobNamespace)
	}

	return nil
}

// newJobRun is a helper which creates the Job related resources from
// the given staging params. That is the job itself, and a secret
// holding the job's environment. Which is a copy of the app
// environment + standard variables.
func newJobRun(app stageParam) (*batchv1.Job, *corev1.Secret) {

	jobName := names.GenerateResourceName("stage", app.Namespace, app.Name, app.Stage.ID)

	// fake stage params of the previous to pull the old image url from.
	previous := app
	previous.Stage = models.NewStage(app.PreviousStageID)

	protocol := "http"
	if app.S3ConnectionDetails.UseSSL {
		protocol = "https"
	}

	// TODO: Simplify env setup -- https://github.com/epinio/epinio/issues/1176

	// runtime: AWSCLIImage
	awsScript := fmt.Sprintf(`PROTOCOL="%s"
ENDPOINT="%s"
BUCKET="%s"
BLOBID="%s"
source "/stage-support/%s"
`,
		protocol, app.S3ConnectionDetails.Endpoint, app.S3ConnectionDetails.Bucket,
		app.BlobUID, helmchart.EpinioStageDownload)

	// runtime: BashImage
	unpackScript := fmt.Sprintf(`BLOBID="%s"
source "/stage-support/%s"
`,
		app.BlobUID, helmchart.EpinioStageUnpack)

	// runtime: app.BuilderImage
	buildpackScript := fmt.Sprintf(`PREIMAGE="%s"
APPIMAGE="%s"
source "/stage-support/%s"`,
		previous.ImageURL(previous.RegistryURL),
		app.ImageURL(app.RegistryURL),
		helmchart.EpinioStageBuild)

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "s3-creds",
			MountPath: "/root/.aws",
			ReadOnly:  true,
		},
		{
			Name:      "source",
			SubPath:   "source",
			MountPath: "/workspace/source",
		},
		{
			Name:      "cache",
			SubPath:   "cache",
			MountPath: "/workspace/cache",
		},
		{
			Name:      "registry-creds",
			MountPath: "/home/cnb/.docker/",
			ReadOnly:  true,
		},
		{
			Name:      "staging",
			MountPath: "/stage-support",
		},
		{
			Name:      "app-environment",
			MountPath: "/platform/appenv",
			ReadOnly:  true,
		},
		{
			Name:      "job-environment",
			MountPath: "/platform/env",
		},
	}

	cacheClaim := &corev1.PersistentVolumeClaimVolumeSource{
		ClaimName: app.MakePVCName(),
		ReadOnly:  false,
	}

	volumes := []corev1.Volume{
		{
			Name: "staging",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: helmchart.EpinioStageScriptsName,
					},
					DefaultMode: pointer.Int32(420),
				},
			},
		},
		{
			// See `jobenv` for the Secret providing the information.
			Name: "app-environment",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  jobName,
					DefaultMode: pointer.Int32(420),
				},
			},
		},
		{
			Name: "job-environment",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: cacheClaim,
			},
		},
		{
			Name: "source",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "s3-creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  helmchart.S3ConnectionDetailsSecretName,
					DefaultMode: pointer.Int32(420),
				},
			},
		},
		{
			Name: "registry-creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  "registry-creds",
					DefaultMode: pointer.Int32(420),
					Items: []corev1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
					},
				},
			},
		},
	}

	// If there is a certificate to trust
	if app.RegistryCASecret != "" && app.RegistryCAHash != "" {

		volumes = append(volumes, corev1.Volume{
			Name: "registry-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  app.RegistryCASecret,
					DefaultMode: pointer.Int32(420),
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "registry-certs",
			MountPath: fmt.Sprintf("/etc/ssl/certs/%s", app.RegistryCAHash),
			SubPath:   "ca.crt",
			ReadOnly:  true,
		})
	}

	// Create job environment as a copy of the app environment, plus standard variable.
	env := make(map[string][]byte)

	env["CNB_PLATFORM_API"] = []byte("0.4")
	for _, ev := range app.Environment {
		env[ev.Name] = []byte(ev.Value)
	}

	jobenv := &corev1.Secret{
		Data: env,
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/part-of":    app.Namespace,
				"app.kubernetes.io/created-by": app.Username,
				models.EpinioStageIDLabel:      app.Stage.ID,
				models.EpinioStageIDPrevious:   app.PreviousStageID,
				models.EpinioStageBlobUIDLabel: app.BlobUID,
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/component":  "staging",
			},
		},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/part-of":    app.Namespace,
				"app.kubernetes.io/created-by": app.Username,
				models.EpinioStageIDLabel:      app.Stage.ID,
				models.EpinioStageIDPrevious:   app.PreviousStageID,
				models.EpinioStageBlobUIDLabel: app.BlobUID,
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/component":  "staging",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: pointer.Int32(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":       app.Name,
						"app.kubernetes.io/part-of":    app.Namespace,
						"app.kubernetes.io/created-by": app.Username,
						models.EpinioStageIDLabel:      app.Stage.ID,
						models.EpinioStageIDPrevious:   app.PreviousStageID,
						models.EpinioStageBlobUIDLabel: app.BlobUID,
						"app.kubernetes.io/managed-by": "epinio",
						"app.kubernetes.io/component":  "staging",
					},
					Annotations: map[string]string{
						// Allow communication with the Registry even before the proxy is ready
						"config.linkerd.io/skip-outbound-ports": "443",
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:         "download-s3-blob",
							Image:        AWSCLIImage,
							VolumeMounts: volumeMounts,
							Command:      []string{"/bin/bash"},
							Args: []string{
								"-c",
								awsScript,
							},
						},
						{
							Name:         "unpack-blob",
							Image:        BashImage,
							VolumeMounts: volumeMounts,
							Command:      []string{"bash"},
							Args: []string{
								"-c",
								unpackScript,
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "buildpack",
							Image:   app.BuilderImage,
							Command: []string{"/bin/bash"},
							Args: []string{
								"-c",
								buildpackScript,
							},
							VolumeMounts: volumeMounts,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  pointer.Int64(1000),
								RunAsGroup: pointer.Int64(1000),
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       volumes,
				},
			},
		},
	}

	return job, jobenv
}

func getRegistryURL(ctx context.Context, cluster *kubernetes.Cluster) (string, error) {
	cd, err := registry.GetConnectionDetails(ctx, cluster, helmchart.StagingNamespace, registry.CredentialsSecretName)
	if err != nil {
		return "", err
	}
	registryPublicURL, err := cd.PublicRegistryURL()
	if err != nil {
		return "", err
	}
	if registryPublicURL == "" {
		return "", errors.New("no public registry URL found")
	}

	return fmt.Sprintf("%s/%s", registryPublicURL, cd.Namespace), nil
}

// The equivalent of:
// kubectl get secret -n (helmchart.StagingNamespace) epinio-registry-tls -o json | jq -r '.["data"]["tls.crt"]' | base64 -d | openssl x509 -hash -noout
// written in golang.
func getRegistryCertificateHash(ctx context.Context, c *kubernetes.Cluster, namespace string, name string) (string, error) {
	secret, err := c.Kubectl.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// cert-manager doesn't add the CA for ACME certificates:
	// https://github.com/jetstack/cert-manager/issues/2111
	if _, found := secret.Data["tls.crt"]; !found {
		return "", nil
	}

	hash, err := cahash.GenerateHash(secret.Data["tls.crt"])
	if err != nil {
		return "", err
	}

	return hash, nil
}

// getBuilderImage returns the builder image defined on the request. If that
// one is not defined, it tries to find the builder image previously used on the
// Application CR. If one is not found, it returns an error.
func getBuilderImage(req models.StageRequest, app *unstructured.Unstructured) (string, apierror.APIErrors) {
	var returnErr apierror.APIErrors

	if req.BuilderImage != "" {
		return req.BuilderImage, nil
	}

	builderImage, _, err := unstructured.NestedString(app.UnstructuredContent(), "spec", "builderimage")
	if err != nil {
		returnErr = apierror.InternalError(err, "builderimage should be a string!")
		return "", returnErr
	}

	if builderImage == "" {
		returnErr = apierror.NewBadRequest("request didn't provide a builder image and a previous one doesn't exist")
		return "", returnErr
	}

	return builderImage, nil
}

func getBlobUID(ctx context.Context, s3ConnectionDetails s3manager.ConnectionDetails, req models.StageRequest, app *unstructured.Unstructured) (string, apierror.APIErrors) {
	var blobUID string
	var err error
	var returnErr apierror.APIErrors

	if req.BlobUID != "" {
		blobUID = req.BlobUID
	} else {
		blobUID, err = findPreviousBlobUID(app)
		if err != nil {
			returnErr = apierror.InternalError(err, "looking up the previous blod UID")
			return "", returnErr
		}
	}

	if blobUID == "" {
		returnErr = apierror.NewBadRequest("request didn't provide a blobUID and a previous one doesn't exist")
		return "", returnErr
	}

	// Validate incoming blob id before attempting to stage
	apierr := validateBlob(ctx, blobUID, req.App, s3ConnectionDetails)
	if apierr != nil {
		return "", apierr
	}

	return blobUID, nil
}

func findPreviousBlobUID(app *unstructured.Unstructured) (string, error) {
	blobUID, _, err := unstructured.NestedString(app.UnstructuredContent(), "spec", "blobuid")
	if err != nil {
		return "", errors.New("blobuid should be string")
	}

	return blobUID, nil
}

func updateApp(ctx context.Context, cluster *kubernetes.Cluster, app *unstructured.Unstructured, params stageParam) error {
	if err := unstructured.SetNestedField(app.Object, params.BlobUID, "spec", "blobuid"); err != nil {
		return err
	}
	if err := unstructured.SetNestedField(app.Object, params.BuilderImage, "spec", "builderimage"); err != nil {
		return err
	}

	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	namespace, _, err := unstructured.NestedString(app.UnstructuredContent(), "metadata", "namespace")
	if err != nil {
		return err
	}

	_, err = client.Namespace(namespace).Update(ctx, app, metav1.UpdateOptions{})

	return err
}
