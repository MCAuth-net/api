package helpers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/argon2"
)

func GenerateToken() (string, error) {
	b := make([]byte, 32)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func GenerateMnemonic() (string, error) {
	words := bip39.GetWordList()

	first, err := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	if err != nil {
		return "", err
	}

	second, err := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	if err != nil {
		return "", err
	}

	return words[first.Int64()] + "-" + words[second.Int64()], nil
}

const (
	memory      = 65536
	iterations  = 4
	parallelism = 2
	saltLength  = 16
	keyLength   = 32
)

func Hash(password string) (string, error) {
	salt := make([]byte, saltLength)

	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		keyLength,
	)

	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory,
		iterations,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}
func Compare(password, encodedHash string) (bool, error) {
	var version, memory, iterations, parallelism uint32
	var salt, storedHash string

	_, err := fmt.Sscanf(
		encodedHash,
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		&version,
		&memory,
		&iterations,
		&parallelism,
		&salt,
		&storedHash,
	)
	if err != nil || version != argon2.Version {
		return false, fmt.Errorf("invalid argon2 hash")
	}

	saltBytes, err := base64.RawStdEncoding.DecodeString(salt)
	if err != nil {
		return false, err
	}

	hashBytes, err := base64.RawStdEncoding.DecodeString(storedHash)
	if err != nil {
		return false, err
	}

	if len(hashBytes) == 0 || len(saltBytes) == 0 ||
		memory < 8*parallelism || memory > 256*1024 ||
		iterations == 0 || iterations > 10 ||
		parallelism == 0 || parallelism > 16 {
		return false, fmt.Errorf("invalid argon2 parameters")
	}

	comparisonHash := argon2.IDKey(
		[]byte(password),
		saltBytes,
		iterations,
		memory,
		uint8(parallelism),
		uint32(len(hashBytes)),
	)

	return subtle.ConstantTimeCompare(hashBytes, comparisonHash) == 1, nil
}
