package address

import "math/big"

// Deriver derives a blockchain address from an ECDSA secp256k1 public key.
type Deriver interface {
	Derive(x, y *big.Int) string
}

var registry = map[string]Deriver{
	"ethereum": ethereumDeriver{},
	"bitcoin":  bitcoinDeriver{},
}

// Derive returns the address for the given chain name, and false if unsupported.
func Derive(chain string, x, y *big.Int) (string, bool) {
	d, ok := registry[chain]
	if !ok {
		return "", false
	}
	return d.Derive(x, y), true
}

// All derives addresses for every registered chain.
func All(x, y *big.Int) map[string]string {
	result := make(map[string]string, len(registry))
	for name, d := range registry {
		result[name] = d.Derive(x, y)
	}
	return result
}

// padTo32 left-pads a big.Int to a 32-byte big-endian slice.
func padTo32(n *big.Int) []byte {
	b := n.Bytes()
	if len(b) >= 32 {
		return b
	}
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return padded
}

// compressedPubKey returns the 33-byte SEC-compressed public key.
func compressedPubKey(x, y *big.Int) []byte {
	prefix := byte(0x02)
	if y.Bit(0) == 1 {
		prefix = 0x03
	}
	out := make([]byte, 33)
	out[0] = prefix
	copy(out[1:], padTo32(x))
	return out
}
