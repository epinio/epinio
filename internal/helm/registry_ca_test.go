package helm

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
)

var _ = Describe("rootCAsWith", func() {
	var (
		ca    *x509.Certificate
		caPEM []byte
	)

	BeforeEach(func() {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).ToNot(HaveOccurred())

		template := &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "registry-ca"},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(time.Hour),
			IsCA:                  true,
			BasicConstraintsValid: true,
			KeyUsage:              x509.KeyUsageCertSign,
		}
		der, err := x509.CreateCertificate(
			rand.Reader,
			template,
			template,
			&key.PublicKey,
			key,
		)
		Expect(err).ToNot(HaveOccurred())

		ca, err = x509.ParseCertificate(der)
		Expect(err).ToNot(HaveOccurred())
		caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	})

	It("trusts the certificate", func() {
		pool, err := rootCAsWith(caPEM)
		Expect(err).ToNot(HaveOccurred())

		_, err = ca.Verify(x509.VerifyOptions{Roots: pool})
		Expect(err).ToNot(HaveOccurred())
	})

	It("keeps the system CAs", func() {
		system, err := x509.SystemCertPool()
		if err != nil || system.Equal(x509.NewCertPool()) {
			Skip("no system CAs")
		}

		pool, err := rootCAsWith(caPEM)
		Expect(err).ToNot(HaveOccurred())

		Expect(pool.Equal(system)).To(BeFalse())
		system.AddCert(ca)
		Expect(pool.Equal(system)).To(BeTrue())
	})

	It("rejects a secret without certificate", func() {
		_, err := rootCAsWith([]byte("nope"))
		Expect(err).To(HaveOccurred())
	})

	It("adds nothing without a registry certificate secret", func() {
		GinkgoT().Setenv("REGISTRY_CERTIFICATE_SECRET", "")

		pool, err := registryRootCAs(context.Background(), &rest.Config{})
		Expect(err).ToNot(HaveOccurred())
		Expect(pool).To(BeNil())
	})
})
