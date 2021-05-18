package gitea

import (
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"time"

	giteaSDK "code.gitea.io/sdk/gitea"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/pkg/errors"
)

const LocalRegistry = "127.0.0.1:30500/apps"

// Upload puts the app data into the gitea repo and creates the webhook and
// accompanying app data.
// The results are added to the struct App.
func (c *Client) Upload(app *models.App, tmpDir string) error {
	org := app.Org
	name := app.Name

	err := c.createRepo(org, name)
	if err != nil {
		return errors.Wrap(err, "failed to create application")
	}

	u, err := url.Parse(c.URL)
	if err != nil {
		return errors.Wrap(err, "failed to parse gitea url")
	}
	u.User = url.UserPassword(c.Username, c.Password)
	u.Path = path.Join(u.Path, app.Org, app.Name)

	// mutates app and sets repo.revision
	rev, err := c.gitPush(u.String(), tmpDir)
	if err != nil {
		return errors.Wrap(err, "failed to get latest app commit")
	}

	app.Git = &models.GitRef{URL: c.URL, Revision: rev}

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

// gitPush the app data
func (c *Client) gitPush(remote string, tmpDir string) (string, error) {
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
`, tmpDir, remote, time.Now().Format("20060102150405"), "`git branch --show-current`"))

	_, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, "push script failed")
	}

	// extract commit sha
	cmd = exec.Command("/bin/sh", "-c", fmt.Sprintf(`
cd "%s"
git rev-parse HEAD
`, tmpDir))

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, "failed to determine last commit")
	}

	rev := strings.TrimSuffix(string(out), "\n")
	return rev, nil
}
