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

package application

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("extractTar", func() {

	makeTar := func(files map[string]string) *bytes.Buffer {
		buffer := &bytes.Buffer{}
		tarWriter := tar.NewWriter(buffer)
		for name, content := range files {
			header := &tar.Header{
				Name:     name,
				Mode:     0o644,
				Size:     int64(len(content)),
				Typeflag: tar.TypeReg,
			}
			writeHeaderError := tarWriter.WriteHeader(header)
			Expect(writeHeaderError).ToNot(HaveOccurred())
			_, writeError := tarWriter.Write([]byte(content))
			Expect(writeError).ToNot(HaveOccurred())
		}
		closeError := tarWriter.Close()
		Expect(closeError).ToNot(HaveOccurred())
		return buffer
	}

	It("extracts regular files from a plain tar", func() {
		archive := makeTar(map[string]string{
			"main.go":     "package main",
			"sub/util.go": "package sub",
		})

		files, extractError := extractTar(archive)
		Expect(extractError).ToNot(HaveOccurred())
		Expect(files).To(HaveLen(2))
		Expect(string(files["main.go"])).To(Equal("package main"))
		Expect(string(files["sub/util.go"])).To(Equal("package sub"))
	})

	It("extracts regular files from a gzipped tar", func() {
		plain := makeTar(map[string]string{"app.py": "print('hi')"})

		gzipped := &bytes.Buffer{}
		gzipWriter := gzip.NewWriter(gzipped)
		_, writeError := gzipWriter.Write(plain.Bytes())
		Expect(writeError).ToNot(HaveOccurred())
		closeError := gzipWriter.Close()
		Expect(closeError).ToNot(HaveOccurred())

		files, extractError := extractTar(gzipped)
		Expect(extractError).ToNot(HaveOccurred())
		Expect(files).To(HaveLen(1))
		Expect(string(files["app.py"])).To(Equal("print('hi')"))
	})

	It("skips non-regular entries such as directories", func() {
		buffer := &bytes.Buffer{}
		tarWriter := tar.NewWriter(buffer)

		dirHeader := &tar.Header{
			Name:     "subdir/",
			Mode:     0o755,
			Typeflag: tar.TypeDir,
		}
		writeDirError := tarWriter.WriteHeader(dirHeader)
		Expect(writeDirError).ToNot(HaveOccurred())

		fileHeader := &tar.Header{
			Name:     "subdir/file.txt",
			Mode:     0o644,
			Size:     4,
			Typeflag: tar.TypeReg,
		}
		writeFileError := tarWriter.WriteHeader(fileHeader)
		Expect(writeFileError).ToNot(HaveOccurred())
		_, writeError := tarWriter.Write([]byte("data"))
		Expect(writeError).ToNot(HaveOccurred())

		closeError := tarWriter.Close()
		Expect(closeError).ToNot(HaveOccurred())

		files, extractError := extractTar(buffer)
		Expect(extractError).ToNot(HaveOccurred())
		Expect(files).To(HaveLen(1))
		Expect(files).To(HaveKey("subdir/file.txt"))
	})

	It("fails on a corrupt archive", func() {
		_, extractError := extractTar(
			strings.NewReader("this is not a tar archive"),
		)
		Expect(extractError).To(HaveOccurred())
	})

	It("returns an empty map for an empty tar", func() {
		archive := makeTar(map[string]string{})

		files, extractError := extractTar(archive)
		Expect(extractError).ToNot(HaveOccurred())
		Expect(files).To(BeEmpty())
	})
})
