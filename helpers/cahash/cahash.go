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

package cahash

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/paketo-buildpacks/ca-certificates/v3/cacerts"
	"github.com/pkg/errors"
)

// -----------------------------------------------------------------------------------

func GenerateHash(certRaw []byte) (string, error) {
	cert, err := DecodeOneCert(certRaw)
	if err != nil {
		return "", fmt.Errorf("failed to decode certificate\n%w", err)
	}

	hash, err := cacerts.SubjectNameHash(cert)
	if err != nil {
		return "", fmt.Errorf("failed compute subject name hash for cert\n%w", err)
	}

	name := fmt.Sprintf("%08x", hash)
	return name, nil
}

// -----------------------------------------------------------------------------------
// See gh:paketo-buildpacks/ca-certificates (cacerts/certs.go) for the original code.
// https://github.com/paketo-buildpacks/ca-certificates
//
// Iterates over pem blocks until a valid certificate is found or no other PEM blocks exist.
func DecodeOneCert(raw []byte) (*x509.Certificate, error) {
	byteData := raw
	for len(byteData) > 0 {
		block, rest := pem.Decode(byteData)
		if block == nil {
			return nil, errors.New("failed decoding PEM data")
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			byteData = rest
			continue // pem block is not a cert? (e.g. maybe it was a dh_params block)
		}
		return cert, nil
	}

	return nil, errors.New("failed find PEM data")
}

// DecodeCerts iterates over pem blocks and load all the valid certificates
func DecodeCerts(raw []byte) ([]*x509.Certificate, error) {
	certs := []*x509.Certificate{}

	byteData := raw
	for len(byteData) > 0 {
		block, rest := pem.Decode(byteData)
		if block == nil {
			return nil, errors.New("failed decoding PEM data")
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			byteData = rest
			continue // pem block is not a cert? (e.g. maybe it was a dh_params block)
		}

		certs = append(certs, cert)
	}

	return certs, nil
}
