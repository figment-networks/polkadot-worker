package blake128

import (
	"golang.org/x/crypto/blake2b"
)

func ToBlakeSum(address []byte) ([]byte, error) {
	b, err := blake2b.New(16, nil) // it's size of 16 - regardless of whatever polka docs says
	if err != nil {
		return nil, err
	}
	_, err = b.Write(address)
	return b.Sum(nil), err
}
