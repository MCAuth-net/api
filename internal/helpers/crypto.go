package helpers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

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

func GenerateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func SHA256(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
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
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid argon2 hash")
	}

	var version, memory, iterations, parallelism uint32

	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("invalid argon2 version")
	}

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d",
		&memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("invalid argon2 parameters")
	}

	if version != argon2.Version {
		return false, fmt.Errorf("invalid argon2 version")
	}

	saltBytes, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	hashBytes, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	if len(hashBytes) == 0 || len(saltBytes) == 0 ||
		parallelism == 0 || parallelism > 16 ||
		memory < 8*parallelism || memory > 256*1024 ||
		iterations == 0 || iterations > 10 {
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
