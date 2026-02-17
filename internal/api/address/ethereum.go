package address

import (
	"encoding/hex"
	"math/big"

	"golang.org/x/crypto/sha3"
)

type ethereumDeriver struct{}

// Derive returns the EIP-55 checksummed Ethereum address for the given public key.
func (ethereumDeriver) Derive(x, y *big.Int) string {
	// Keccak256 of the uncompressed 64-byte public key (no 0x04 prefix).
	h := sha3.NewLegacyKeccak256()
	h.Write(padTo32(x))
	h.Write(padTo32(y))
	hash := h.Sum(nil)

	return "0x" + eip55Checksum(hash[12:]) // last 20 bytes
}

// eip55Checksum applies EIP-55 mixed-case checksum encoding to a 20-byte address.
func eip55Checksum(addr []byte) string {
	hexAddr := hex.EncodeToString(addr)

	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(hexAddr))
	hash := h.Sum(nil)

	out := []byte(hexAddr)
	for i, c := range out {
		if c >= 'a' && c <= 'f' {
			nibble := hash[i/2]
			if i%2 == 0 {
				nibble >>= 4
			}
			if nibble&0x0f >= 8 {
				out[i] = c - 32 // uppercase
			}
		}
	}
	return string(out)
}
