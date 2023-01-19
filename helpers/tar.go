// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"os"
	"path"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
)

func Tar(dir string) (string, string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", "", errors.Wrap(err, "cannot read the apps source files")
	}
	sources := []string{}
	for _, f := range files {
		// The os.DirEntry structures returned by ReadDir() provide only the base
		// name of the file or directory they are for. We have to add back the
		// path of the application directory to get the proper paths to the files
		// and directories to assemble in the tarball.

		// Ignore git config files in the app sources.
		if f.Name() == ".git" || f.Name() == ".gitignore" || f.Name() == ".gitmodules" || f.Name() == ".gitconfig" || f.Name() == ".git-credentials" {
			continue
		}
		sources = append(sources, path.Join(dir, f.Name()))
	}

	// create a tmpDir - tarball dir and POST
	tmpDir, err := os.MkdirTemp("", "epinio-app")
	if err != nil {
		return "", "", errors.Wrap(err, "can't create temp directory")
	}

	tarball := path.Join(tmpDir, "blob.tar")
	err = archiver.Archive(sources, tarball)
	if err != nil {
		return tmpDir, "", errors.Wrap(err, "can't create archive")
	}

	return tmpDir, tarball, nil
}
