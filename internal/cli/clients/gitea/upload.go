package gitea

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"
	"time"

	giteaSDK "code.gitea.io/sdk/gitea"
	"github.com/epinio/epinio/deployments"
	"github.com/pkg/errors"
)

var (
	// HookSecret should be generated
	// TODO: generate this and put it in a secret
	HookSecret = "74tZTBHkhjMT5Klj6Ik6PqmM"

	// StagingEventListenerURL should not exist
	// TODO: detect this based on namespaces and services
	StagingEventListenerURL = "http://el-staging-listener." + deployments.TektonStagingNamespace + ":8080"
)

// CreateApp puts the app data into the gitea repo and creates the webhook and
// accompanying app data.
func (c *Client) CreateApp(org string, name string, tmpDir string) error {
	err := c.createRepo(org, name)
	if err != nil {
		return errors.Wrap(err, "failed to create application")
	}

	err = c.createRepoWebhook(org, name)
	if err != nil {
		return errors.Wrap(err, "webhook configuration failed")
	}

	route := c.AppDefaultRoute(name)

	// prepareCode
	err = os.MkdirAll(filepath.Join(tmpDir, ".kube"), 0700)
	if err != nil {
		return errors.Wrap(err, "failed to setup kube resources directory in temp app location")
	}

	if err := c.renderDeployment(filepath.Join(tmpDir, ".kube", "app.yml"), org, name, route); err != nil {
		return err
	}

	if err := renderService(filepath.Join(tmpDir, ".kube", "service.yml"), org, name); err != nil {
		return err
	}

	if err := renderIngress(filepath.Join(tmpDir, ".kube", "ingress.yml"), org, name, route); err != nil {
		return err
	}

	// gitPush
	// TODO c.ui.Normal().Msg("Pushing application code ...")
	u, err := url.Parse(c.URL)
	if err != nil {
		return errors.Wrap(err, "failed to parse gitea url")
	}

	u.User = url.UserPassword(c.Username, c.Password)
	u.Path = path.Join(u.Path, org, name)

	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(`
cd "%s" 
git init
git config user.name "Epinio"
git config user.email ci@epinio
git remote add epinio "%s"
git fetch --all
git reset --soft epinio/main
git add --all
git commit -m "pushed at %s"
git push epinio %s:main
`, tmpDir, u.String(), time.Now().Format("20060102150405"), "`git branch --show-current`"))

	_, err = cmd.CombinedOutput()
	if err != nil {
		// TODO c.ui.Problem().
		//         WithStringValue("Stdout", string(output)).
		//         WithStringValue("Stderr", "").
		//         Msg("App push failed")
		return errors.Wrap(err, "push script failed")
	}

	// TODO c.ui.Note().V(1).WithStringValue("Output", string(output)).Msg("")
	// c.ui.Success().Msg("Application push successful")

	return nil
}

func (c *Client) AppDefaultRoute(name string) string {
	return fmt.Sprintf("%s.%s", name, c.Domain)
}

func (c *Client) createRepo(org string, name string) error {
	_, resp, err := c.Client.GetRepo(org, name)
	if resp == nil && err != nil {
		return errors.Wrap(err, "failed to make get repo request")
	}

	if resp.StatusCode == 200 {
		// TODO c.ui.Note().Msg("Application already exists. Updating.")
		return nil
	}

	_, _, err = c.Client.CreateOrgRepo(org, giteaSDK.CreateRepoOption{
		Name:          name,
		AutoInit:      true,
		Private:       true,
		DefaultBranch: "main",
	})

	if err != nil {
		return errors.Wrap(err, "failed to create application")
	}

	return nil
}

func (c *Client) createRepoWebhook(org string, name string) error {
	hooks, _, err := c.Client.ListRepoHooks(org, name, giteaSDK.ListHooksOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list webhooks")
	}

	for _, hook := range hooks {
		url := hook.Config["url"]
		if url == StagingEventListenerURL {
			// TODO c.ui.Normal().Msg("Webhook already exists.")
			return nil
		}
	}

	// TODO c.ui.Normal().Msg("Creating webhook in the repo...")
	c.Client.CreateRepoHook(org, name, giteaSDK.CreateHookOption{
		Active:       true,
		BranchFilter: "*",
		Config: map[string]string{
			"secret":       HookSecret,
			"http_method":  "POST",
			"url":          StagingEventListenerURL,
			"content_type": "json",
		},
		Type: "gitea",
	})

	return nil
}

func (c *Client) renderDeployment(filePath string, org string, appName string, route string) error {
	deploymentTmpl, err := template.New("deployment").Parse(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "{{ .AppName }}"
  namespace: {{ .Org }}
  labels:
    app.kubernetes.io/name: "{{ .AppName }}"
    app.kubernetes.io/part-of: "{{ .Org }}"
    app.kubernetes.io/component: application
    app.kubernetes.io/managed-by: epinio
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: "{{ .AppName }}"
  template:
    metadata:
      labels:
        app.kubernetes.io/name: "{{ .AppName }}"
        app.kubernetes.io/part-of: "{{ .Org }}"
        app.kubernetes.io/component: application
        app.kubernetes.io/managed-by: epinio
      annotations:
        app.kubernetes.io/name: "{{ .AppName }}"
    spec:
      # TODO: Do these when you create an org
      serviceAccountName: {{ .Org }}
      automountServiceAccountToken: false
      containers:
      - name: "{{ .AppName }}"
        image: "127.0.0.1:30500/apps/{{ .AppName }}"
        ports:
        - containerPort: 8080
        env:
        - name: PORT
          value: "8080"
  `)
	if err != nil {
		return errors.Wrap(err, "failed to parse deployment template for app")
	}

	appFile, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create file for kube resource definitions")
	}
	defer func() { err = appFile.Close() }()

	err = deploymentTmpl.Execute(appFile, struct {
		AppName string
		Route   string
		Org     string
	}{
		AppName: appName,
		Route:   route,
		Org:     org,
	})

	if err != nil {
		return errors.Wrap(err, "failed to render kube resource definition")
	}

	return nil
}

func renderService(filePath, org string, appName string) error {
	serviceTmpl, err := template.New("service").Parse(`
apiVersion: v1
kind: Service
metadata:
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls: "true"
  labels:
    app.kubernetes.io/component: application
    app.kubernetes.io/managed-by: epinio
    app.kubernetes.io/name: {{ .AppName }}
    app.kubernetes.io/part-of: {{ .Org }}
    kubernetes.io/ingress.class: traefik
  name: {{ .AppName }}
  namespace: {{ .Org }}
spec:
  ports:
  - port: 8080
    protocol: TCP
    targetPort: 8080
  selector:
    app.kubernetes.io/name: "{{ .AppName }}"
  type: ClusterIP
  `)
	if err != nil {
		return errors.Wrap(err, "failed to parse service template for app")
	}

	serviceFile, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create file for application Service definition")
	}
	defer func() { err = serviceFile.Close() }()

	err = serviceTmpl.Execute(serviceFile, struct {
		AppName string
		Org     string
	}{
		AppName: appName,
		Org:     org,
	})
	if err != nil {
		return errors.Wrap(err, "failed to render application Service definition")
	}

	return nil
}

func renderIngress(filePath, org, appName, route string) error {
	ingressTmpl, err := template.New("ingress").Parse(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls: "true"
  labels:
    app.kubernetes.io/component: application
    app.kubernetes.io/managed-by: epinio
    app.kubernetes.io/name: {{ .AppName }}
    app.kubernetes.io/part-of: {{ .Org }}
    kubernetes.io/ingress.class: traefik
  name: {{ .AppName }}
  namespace: {{ .Org }}
spec:
  rules:
  - host: {{ .Route }}
    http:
      paths:
      - backend:
          service:
            name: {{ .AppName }}
            port:
              number: 8080
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - {{ .Route }}
    secretName: {{ .AppName }}-tls
  `)
	if err != nil {
		return errors.Wrap(err, "failed to parse ingress template for app")
	}

	ingressFile, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create file for application Ingress definition")
	}
	defer func() { _ = ingressFile.Close() }()

	err = ingressTmpl.Execute(ingressFile, struct {
		AppName string
		Org     string
		Route   string
	}{
		AppName: appName,
		Org:     org,
		Route:   route,
	})
	if err != nil {
		return errors.Wrap(err, "failed to render application Ingress definition")
	}

	return nil
}
