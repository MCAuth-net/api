package helpers

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"

	"github.com/tyler-smith/go-bip39"
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
