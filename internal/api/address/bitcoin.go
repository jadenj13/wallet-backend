package address

import (
	"crypto/sha256"
	"math/big"

	"golang.org/x/crypto/ripemd160" //nolint:staticcheck
)

type bitcoinDeriver struct{}

// Derive returns the Base58Check-encoded Bitcoin P2PKH mainnet address.
func (bitcoinDeriver) Derive(x, y *big.Int) string {
	compressed := compressedPubKey(x, y)

	// Hash160: RIPEMD160(SHA256(pubkey))
	s := sha256.Sum256(compressed)
	r := ripemd160.New()
	r.Write(s[:])
	hash160 := r.Sum(nil)

	// version(1) + hash160(20)
	payload := make([]byte, 21)
	payload[0] = 0x00 // mainnet P2PKH
	copy(payload[1:], hash160)

	// 4-byte double-SHA256 checksum
	cs := dsha256(payload)
	full := append(payload, cs[:4]...)

	return base58Encode(full)
}

func dsha256(b []byte) []byte {
	first := sha256.Sum256(b)
	second := sha256.Sum256(first[:])
	return second[:]
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func base58Encode(input []byte) string {
	n := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	mod := new(big.Int)

	var result []byte
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		result = append([]byte{base58Alphabet[mod.Int64()]}, result...)
	}

	// Each leading zero byte encodes as '1'.
	for _, b := range input {
		if b != 0x00 {
			break
		}
		result = append([]byte{'1'}, result...)
	}

	return string(result)
}
