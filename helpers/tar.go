// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package helpers

import (
	"context"
	"os"
	"path"

	"github.com/mholt/archives"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

const blobName = "blob.tar"

var excludedGitFiles = []string{
	".git",
	".gitignore",
	".gitmodules",
	".gitconfig",
	".git-credentials",
}

// Creates tar of a git repository directory using mholdt/archives.
func Tar(dir string) (string, string, error) {
	ctx := context.TODO()

	files, err := os.ReadDir(dir)
	if err != nil {
		return "", "", errors.Wrap(err, "cannot read the apps source files")
	}

	sources := make(map[string]string)
	for _, f := range files {
		// Ignore git config files in the app sources.
		if slices.Contains(excludedGitFiles, f.Name()) {
			continue
		}
		
		// The os.DirEntry structures returned by ReadDir() provide only the base
		// name of the file or directory they are for. We have to add back the
		// path of the application directory to get the proper paths to the files
		// and directories to assemble in the tarball.
		//
		// Note that the files and directories must be in a map[string]string format.
		// The second string in the map is the name of the file or directory that 
		// will be reflected in the tarball. For further information, see: 
		// https://github.com/mholt/archives?tab=readme-ov-file#create-archive
		sources[path.Join(dir, f.Name())] = f.Name()
	}
	
	filesFromDisk, filesFromDiskError := archives.FilesFromDisk(
		ctx,
		nil,
		sources,
	)
	if filesFromDiskError != nil {
		return "", "", errors.Wrap(filesFromDiskError, "can't create files from disk for tarball")
	}

	// create a tmpDir - tarball dir and POST
	tmpDir, err := os.MkdirTemp("", "epinio-app")
	tarballName := path.Join(tmpDir, blobName)

	// Open file reference to the tarball file
	outFile, outFileError := os.Create(tarballName)
	if outFileError != nil {
		return tmpDir, "", errors.Wrap(outFileError, "can't create output file")
	}

	defer func () {
		if closeErr := outFile.Close(); closeErr != nil {
			_ = errors.Wrap(closeErr, "can't close output file")
		}
	}()

	// For this case, we just care about the Archival option, but archives supplies
	// more attributes to set here: https://pkg.go.dev/github.com/mholt/archives#CompressedArchive
	// if we need to get more agessive with the compression.
	format := archives.CompressedArchive{
		Archival: archives.Tar{},
	}
	
	writerError := format.Archive(ctx, outFile, filesFromDisk)
	if writerError != nil {
		return tmpDir, "", errors.Wrap(writerError, "can't create tar writer")
	}

	return tmpDir, tarballName, nil
}
