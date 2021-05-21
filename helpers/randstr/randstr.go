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
