// Copyright © 2021 - 2026 KRUMWARE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dex_test

import (
	"github.com/epinio/epinio/internal/dex"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewConfigFromSecretData", func() {
	secretData := map[string][]byte{
		"issuer":         []byte("https://auth.example.com"),
		"endpoint":       []byte("http://dex.epinio.svc.cluster.local:5556"),
		"uiClientSecret": []byte("super-secret"),
	}

	It("loads uiClientSecret only for the epinio-ui client", func() {
		uiCfg, err := dex.NewConfigFromSecretData("epinio-ui", secretData)
		Expect(err).ToNot(HaveOccurred())
		Expect(uiCfg.ClientID).To(Equal("epinio-ui"))
		Expect(uiCfg.ClientSecret).To(Equal("super-secret"))
		Expect(uiCfg.Issuer).To(Equal("https://auth.example.com"))
	})

	It("does not load uiClientSecret for epinio-api", func() {
		apiCfg, err := dex.NewConfigFromSecretData("epinio-api", secretData)
		Expect(err).ToNot(HaveOccurred())
		Expect(apiCfg.ClientID).To(Equal("epinio-api"))
		Expect(apiCfg.ClientSecret).To(BeEmpty())
	})
})
