package gitea

import (
	"github.com/suse/carrier/shim/pkg/shell"
)

// Delete deletes an app
func Delete(info PushInfo) error {
	_, err := shell.ExecTemplate(DeleteAppScript, info)
	if err != nil {
		return err
	}

	return nil
}
