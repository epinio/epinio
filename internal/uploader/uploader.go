package uploader

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/internal/registry"

	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
)

const (
	SourceCodeMediaType = "source.code.epinio.io"
)

type Uploader struct {
	ConnectionDetails registry.ConnectionDetails
}

// TODO: What is the host param?
func (u Uploader) hosts(host string) ([]docker.RegistryHost, error) {
	// TODO: Add our own registry with auth etc using ConnectionDetails
	return docker.ConfigureDefaultRegistries()(host)
}

// Upload uploads an artifact to the OCI registry. It return a UID
// which can be used to download the file or an error if one occurs.
func (u Uploader) Upload(ctx context.Context, file io.Reader) (string, error) {
	ref := "localhost:5000/oras:test"
	blobUID, err := randstr.Hex16()
	if err != nil {
		return "", err
	}

	// resolver := docker.NewResolver(docker.ResolverOptions{
	// 	Hosts: u.hosts,
	// })

	// TODO: See if we can "stream" the file

	tmpFileDir, tmpFileName, err := u.storeTmpFile(file)
	if err != nil {
		return "", nil
	}

	fileStore := content.NewFile(tmpFileDir)
	defer fileStore.Close()

	desc, err := fileStore.Add(blobUID, SourceCodeMediaType, tmpFileName)
	if err != nil {
		return "", err
	}

	manifest, manifestDesc, config, configDesc, err := content.GenerateManifestAndConfig(nil, nil, desc)
	if err != nil {
		return "", err
	}

	fileStore.StoreManifest(ref, configDesc, config)
	if err := fileStore.StoreManifest(ref, manifestDesc, manifest); err != nil {
		return "", err
	}

	registry, err := content.NewRegistry(content.RegistryOptions{PlainHTTP: true})
	if err != nil {
		return "", err
	}

	fmt.Printf("Pushing %s to %s...\n", blobUID, ref)
	desc, err = oras.Copy(ctx, fileStore, ref, registry, "")

	return "", err
}

// Stores the given io.Reader to a temporary file and returns the tmp directory
// and the file name.
func (u Uploader) storeTmpFile(file io.Reader) (string, string, error) {
	tmpfile, err := ioutil.TempFile("", "application-src-")
	if err != nil {
		return "", "", err
	}

	filePath := tmpfile.Name()
	directory := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	_, err = tmpfile.ReadFrom(file)
	if err != nil {
		return directory, fileName, err
	}

	if err := tmpfile.Close(); err != nil {
		return directory, fileName, err
	}

	return directory, fileName, nil
}
