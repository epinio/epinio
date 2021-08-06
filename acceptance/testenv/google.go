package testenv

import (
	"os"
	"regexp"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/helpers"
	"github.com/pkg/errors"
)

func SetupGoogleServices(epinioBinary string) error {
	serviceAccountJSON, err := helpers.CreateTmpFile(`
				{
					"type": "service_account",
					"project_id": "myproject",
					"private_key_id": "somekeyid",
					"private_key": "someprivatekey",
					"client_email": "client@example.com",
					"client_id": "clientid",
					"auth_uri": "https://accounts.google.com/o/oauth2/auth",
					"token_uri": "https://oauth2.googleapis.com/token",
					"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/client%40example.com"
				}
			`)
	if err != nil {
		return errors.Wrap(err, serviceAccountJSON)
	}

	defer os.Remove(serviceAccountJSON)

	out, err := proc.Run("", false, Root()+epinioBinary, "enable", "services-google",
		"--service-account-json", serviceAccountJSON)
	if err != nil {
		return errors.Wrap(err, out)
	}

	out, err = helpers.Kubectl("get", "pods",
		"--namespace", "google-service-broker",
		"--selector", "app.kubernetes.io/name=gcp-service-broker")
	if err != nil {
		return errors.Wrap(err, out)
	}
	m, err := regexp.MatchString(`google-service-broker-gcp-service-broker.*2/2.*Running`, out)
	if err != nil {
		return err
	}
	if !m {
		panic(m)
	}
	return nil
}
