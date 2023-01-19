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

// Package randstr provides functions for the generation of random strings.
// Useful, for example, to generate passwords and usernames for randomized credentials.
package randstr

import (
	"crypto/rand"
	"encoding/hex"
	"hash/fnv"
)

func Hex16() (string, error) {
	randBytes := make([]byte, 16)
	_, err := rand.Read(randBytes)
	if err != nil {
		return "", err
	}

	a := fnv.New64()
	_, err = a.Write([]byte(string(randBytes)))
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(a.Sum(nil)), nil
}
