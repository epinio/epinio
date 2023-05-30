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

package cahash_test

import (
	"github.com/epinio/epinio/helpers/cahash"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GenerateHash", func() {

	When("cert is from a CA", func() {
		It("will be decoded ok", func() {
			hash, err := cahash.GenerateHash([]byte(caCrt))
			Expect(err).ToNot(HaveOccurred())
			Expect(hash).ToNot(BeNil())
			Expect(hash).To(Equal("4a2d0bdb"))
		})
	})

	When("cert has CA:TRUE in Key Usage", func() {
		It("will be decoded ok", func() {
			hash, err := cahash.GenerateHash([]byte(ca2Crt))
			Expect(err).ToNot(HaveOccurred())
			Expect(hash).ToNot(BeNil())
			Expect(hash).To(Equal("bb7fe9fe"))
		})
	})

	When("cert is not a CA certificate", func() {
		It("will be decoded ok", func() {
			hash, err := cahash.GenerateHash([]byte(tlsCert))
			Expect(err).ToNot(HaveOccurred())
			Expect(hash).ToNot(BeNil())
			Expect(hash).To(Equal("eea339da"))
		})
	})

	When("PEM data does not contain a certificate", func() {
		It("returns an error", func() {
			hash, err := cahash.GenerateHash([]byte(dhParams))
			Expect(err).To(HaveOccurred())
			Expect(hash).To(Equal(""))
		})
	})

	When("byte data are not PEM encoded", func() {
		It("returns an error", func() {
			hash, err := cahash.GenerateHash([]byte("jnhsdjknjsdk"))
			Expect(err).To(HaveOccurred())
			Expect(hash).To(Equal(""))
		})
	})

})

var (
	caCrt = `-----BEGIN CERTIFICATE-----
MIIBaDCCAQ6gAwIBAgIQfKPL2KSrus0tgwC+yXWCCjAKBggqhkjOPQQDAjAUMRIw
EAYDVQQDEwllcGluaW8tY2EwHhcNMjMwNTI5MDk0MDEyWhcNMjMwODI3MDk0MDEy
WjAUMRIwEAYDVQQDEwllcGluaW8tY2EwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNC
AATAGx1D7ouV5MHIUWFwP9yaH4uu65kGP9F9wLden/6Kt5OLOA2R5D8IMSYfg5A2
M+QsXBNefq/nYylMxwrAglC4o0IwQDAOBgNVHQ8BAf8EBAMCAqQwDwYDVR0TAQH/
BAUwAwEB/zAdBgNVHQ4EFgQUoAS25Dc5P4gvlo5GJ7gTi0MNrmMwCgYIKoZIzj0E
AwIDSAAwRQIgLqCFhVfWJb9pRTNRYTpJ0VHi1YIuhbzKKt/PMUN5N4ECIQDhTYcX
MoTdO/129uqhd26+7dODT6zSF2sBYGS17IvdmA==
-----END CERTIFICATE-----
`

	ca2Crt = `-----BEGIN CERTIFICATE-----
MIIE1zCCA7+gAwIBAgIUSDJy88GCPAKMX961dozA/Fk56ocwDQYJKoZIhvcNAQEL
BQAwgZoxCzAJBgNVBAYTAkNaMQ4wDAYDVQQIDAVDemVjaDEQMA4GA1UEBwwHUHJh
Z3VlcjENMAsGA1UECgwEU1VTRTEPMA0GA1UECwwGZXBpbmlvMScwJQYDVQQDDB50
aC1yZWdpc3RyeS5jaS5lcGluaW8uc3VzZS5kZXYxIDAeBgkqhkiG9w0BCQEWEXRo
ZWhlamlrQHN1c2UuY29tMB4XDTIzMDUyOTA5Mjc1OVoXDTMzMDUyNjA5Mjc1OVow
gZoxCzAJBgNVBAYTAkNaMQ4wDAYDVQQIDAVDemVjaDEQMA4GA1UEBwwHUHJhZ3Vl
cjENMAsGA1UECgwEU1VTRTEPMA0GA1UECwwGZXBpbmlvMScwJQYDVQQDDB50aC1y
ZWdpc3RyeS5jaS5lcGluaW8uc3VzZS5kZXYxIDAeBgkqhkiG9w0BCQEWEXRoZWhl
amlrQHN1c2UuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAr9Es
UywTXXGPQ1H57SxIX/JzqgBnwPtqZlRFss7bddGO/oJtIhu806tcn60pVcYtfJa4
xiz/tCw2YB8LoJNjMO56zSU74t+IJu367jwA0KC88Ym5RjUoZX0QC5xQYQv5dVig
TgZHyxc3Pp3Gq3+eHX5tEBKqnzHuYM3fHcbzBvtZ/mcBWo4NK+S0y/787Y3GzfmP
PsUDcSVs/zcWD538JGbjVca/HYXaFon6oy44l3ZcFN6IiUZis1rTDLDMuD7IT+36
NZu4oGk53xmVQv8sa/JEFtPAqvKLk2dhSBpIym6lKMr+vrLBAaIavjacaAwAkvII
MSU5SxZ29RhXUfinJQIDAQABo4IBETCCAQ0wgcQGA1UdIwSBvDCBuaGBoKSBnTCB
mjELMAkGA1UEBhMCQ1oxDjAMBgNVBAgMBUN6ZWNoMRAwDgYDVQQHDAdQcmFndWVy
MQ0wCwYDVQQKDARTVVNFMQ8wDQYDVQQLDAZlcGluaW8xJzAlBgNVBAMMHnRoLXJl
Z2lzdHJ5LmNpLmVwaW5pby5zdXNlLmRldjEgMB4GCSqGSIb3DQEJARYRdGhlaGVq
aWtAc3VzZS5jb22CFEgycvPBgjwCjF/etXaMwPxZOeqHMAwGA1UdEwQFMAMBAf8w
CwYDVR0PBAQDAgTwMCkGA1UdEQQiMCCCHnRoLXJlZ2lzdHJ5LmNpLmVwaW5pby5z
dXNlLmRldjANBgkqhkiG9w0BAQsFAAOCAQEAAa4GsrbdvNKsDUTEy2EQIUPlLcJt
cKnJNByDuvVsObN9/o161OlqIo7pPq02xsrbSWTECtaPbKs4JAw3l8jTu7seQ92p
bikAGrN4jbMHkLzoltdGQ6wI/qAaEJ18vSAnvHLE0mClC8/UX45oteMO9y0oOPMf
amYOhy46EVjwhMQDbMGVtqLDTYV4Wh/FOlf0k0SNS/0YwA03EvMlVV6YuCe15NA/
KsBNWqAwpXLT7hnbs318Tq1MpUnS7SGWS+NQO8lA8FWz8QxEn/hQnXkQZ1xhVJ3Q
gTx1164+0b5waF86X3PQEVSM4qCNnNBS6CB6kJNGzjE6Ulx+D6HVpOwj0A==
-----END CERTIFICATE-----`

	tlsCert = `-----BEGIN CERTIFICATE-----
MIICVjCCAfugAwIBAgIQb+dL7bataJBnRIpe6Y51UjAKBggqhkjOPQQDAjAUMRIw
EAYDVQQDEwllcGluaW8tY2EwHhcNMjMwNTI5MDk0MDE3WhcNMjMwODI3MDk0MDE3
WjAAMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA1M3KAUzBqUVWxggg
e6XsThNt6gYzlgnMUdHtza/TsBaDEJznI8u8/ppL+tp58IB6UFGHXkJfZHYMhDb8
xSc4031RevF7W7Y1rKmoFQekN2HWJM2WazWiznar5b+MrYo7CIhFVH4lui1cqBEL
YIJPPOtEU+E9DROWzSsobmlDfbTSys9suVNTKE9orrU7t6tpvZ3dy6OdQyrxeggF
Gzr+Xe+sGab7+bEwCG0DjHQ1ap1gV730DJah/mIZr+MzVbysCZH54gM1zgUV/5LA
qnEvia6h6QG8jjV1QZSyXCqUJXeYnijs/h5R6ck4jfST1CzyNJg2R5KEjpfmpF08
ruxc5QIDAQABo3gwdjAOBgNVHQ8BAf8EBAMCBaAwDAYDVR0TAQH/BAIwADAfBgNV
HSMEGDAWgBSgBLbkNzk/iC+WjkYnuBOLQw2uYzA1BgNVHREBAf8EKzApgiFyZWdp
c3RyeS5lcGluaW8uc3ZjLmNsdXN0ZXIubG9jYWyHBH8AAAEwCgYIKoZIzj0EAwID
SQAwRgIhAJhtbr9lMKsgwK7L6jOU5VTc065YrDoEj6/xs9UCdhPCAiEAptsuBGMb
bwsBe8AtriDRezWjmgfs01ScAQkgipp7/HQ=
-----END CERTIFICATE-----
`

	dhParams = `-----BEGIN DH PARAMETERS-----
MIGHAoGBANclzZUHl2R0NYH5D4cIHcfM8ATuk75NeO2iaV3FhcAAfs9ljlOuJaVn
UDH9qdl9A4YrDi3VPm55r/YHA4v3wx42Xaq4YfbljeGOKfT6HuhIVS9/n3ZjwNFe
2IAJeiV4VCRAmjVrgZcUodpEK+jEH4tULNS3NO3p6BbvU/6gyCQLAgEC
-----END DH PARAMETERS-----`
)
