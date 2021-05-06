package address

import "github.com/btcsuite/btcutil/base58"

func Decode(address string) []byte {
	return base58.Decode(address)
}

// CheckAddress - trim base58 properly
func CheckAddress(decoded []byte) (endPos uint64, ss58Length uint8, ss58Decoded byte) {
	ss58Length = 1
	if uint8(decoded[0]&0b01000000) != 0 {
		ss58Length = 2
	}

	ss58Decoded = decoded[0]
	if uint8(ss58Length) == 2 {
		ss58Decoded = (decoded[0]&0b00111111)<<2 |
			decoded[1]>>6 |
			(decoded[1]&0b00111111)<<8
	}

	dLen := uint64(len(decoded))
	endPos = dLen - 1
	if uint64(ss58Length) == dLen-34 || uint64(ss58Length) == dLen-35 { // is public key
		endPos = dLen - 2
	}

	return endPos, uint8(ss58Length), ss58Decoded
}
