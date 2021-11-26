package cahash

import (
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// -----------------------------------------------------------------------------------

func GenerateHash(certRaw []byte) (string, error) {
	cert, err := decodeOneCert(certRaw)
	if err != nil {
		return "", fmt.Errorf("failed to decode certificate\n%w", err)
	}

	hash, err := SubjectNameHash(cert)
	if err != nil {
		return "", fmt.Errorf("failed compute subject name hash for cert\n%w", err)
	}

	name := fmt.Sprintf("%08x", hash)
	return name, nil
}

// -----------------------------------------------------------------------------------
// See gh:paketo-buildpacks/ca-certificates (cacerts/certs.go) for the original code.

func decodeOneCert(raw []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("failed find PEM data")
	}
	extra, _ := pem.Decode(rest)
	if extra != nil {
		return nil, errors.New("found multiple PEM blocks, expected exactly one")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certficate\n%w", err)
	}
	return cert, nil
}

// SubjectNameHash is a reimplementation of the X509_subject_name_hash
// in openssl. It computes the SHA-1 of the canonical encoding of the
// certificate's subject name and returns the 32-bit integer
// represented by the first four bytes of the hash using little-endian
// byte order.
func SubjectNameHash(cert *x509.Certificate) (uint32, error) {
	name, err := CanonicalName(cert.RawSubject)
	if err != nil {
		return 0, fmt.Errorf("failed to compute canonical subject name\n%w", err)
	}
	hasher := sha1.New() // nolint:gosec // Required by subject hash specification
	_, err = hasher.Write(name)
	if err != nil {
		return 0, fmt.Errorf("failed to compute sha1sum of canonical subject name\n%w", err)
	}
	sum := hasher.Sum(nil)
	return binary.LittleEndian.Uint32(sum[:4]), nil
}

// canonicalSET holds a of canonicalATVs. Suffix SET ensures it is
// marshaled as a set rather than a sequence by asn1.Marshal.
type canonicalSET []canonicalATV

// canonicalATV is similar to pkix.AttributeTypeAndValue but includes
// tag to ensure all values are marshaled as ASN.1, UTF8String values
type canonicalATV struct {
	Type  asn1.ObjectIdentifier
	Value string `asn1:"utf8"`
}

// CanonicalName accepts a DER encoded subject name and returns a
// "Canonical Encoding" matching that returned by the x509_name_canon
// function in openssl. All string values are transformed with
// CanonicalString and UTF8 encoded and the leading SEQ header is
// removed.
//
// For more information see
// https://stackoverflow.com/questions/34095440/hash-algorithm-for-certificate-crl-directory.
func CanonicalName(name []byte) ([]byte, error) {
	var origSeq pkix.RDNSequence
	_, err := asn1.Unmarshal(name, &origSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject name\n%w", err)
	}
	var result []byte
	for _, origSet := range origSeq {
		var canonSet canonicalSET
		for _, origATV := range origSet {
			origVal, ok := origATV.Value.(string)
			if !ok {
				return nil, errors.New("got unexpected non-string value")
			}
			canonSet = append(canonSet, canonicalATV{
				Type:  origATV.Type,
				Value: CanonicalString(origVal),
			})
		}
		setBytes, err := asn1.Marshal(canonSet)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal canonical name\n%w", err)
		}
		result = append(result, setBytes...)
	}
	return result, nil
}

// CanonicalString transforms the given string. All leading and
// trailing whitespace is trimmed where whitespace is defined as a
// space, formfeed, tab, newline, carriage return, or vertical tab
// character. Any remaining sequence of one or more consecutive
// whitespace characters in replaced with a single ' '.
//
// This is a reimplementation of the asn1_string_canon in openssl
func CanonicalString(s string) string {
	s = strings.TrimLeft(s, " \f\t\n\v")
	s = strings.TrimRight(s, " \f\t\n\v")
	s = strings.ToLower(s)
	return string(regexp.MustCompile(`[[:space:]]+`).ReplaceAll([]byte(s), []byte(" ")))
}
