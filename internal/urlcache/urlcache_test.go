// Copyright © 2023 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package urlcache_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/epinio/epinio/internal/urlcache"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("URL Cache", func() {

	var hits int

	initServer := func(statusCode int, responseBody string) string {
		hits = 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
			fmt.Fprint(w, responseBody)
			hits = hits + 1
		}))
		return srv.URL
	}

	It("passes a local file unchanged", func() {
		here, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		tmpFile, err := os.CreateTemp(here, "urlcache-test")
		Expect(err).ToNot(HaveOccurred())
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		path, errc := urlcache.Get(context.Background(), logger, tmpFile.Name())
		Expect(errc).ToNot(HaveOccurred())
		Expect(hits).To(Equal(0))
		Expect(path).To(Equal(tmpFile.Name()))
	})

	It("fails to fetch a bad url", func() {
		url := initServer(500, `hit is bad`)

		path, errc := urlcache.Get(context.Background(), logger, url)
		Expect(errc).To(HaveOccurred())
		Expect(hits).To(Equal(1))
		Expect(path).To(BeEmpty())
	})

	It("fetches an url once", func() {
		url := initServer(200, `OK`)

		patha, errc := urlcache.Get(context.Background(), logger, url)
		Expect(errc).ToNot(HaveOccurred())
		Expect(hits).To(Equal(1))

		pathb, errc := urlcache.Get(context.Background(), logger, url)
		Expect(errc).ToNot(HaveOccurred())
		Expect(hits).To(Equal(1))
		Expect(patha).To(Equal(pathb))
	})
})
