package crypto

import "runtime"

// WipeBytes securely overwrites a byte slice with zeros to minimize
// the window sensitive cryptographic material remains resident in memory.
func WipeBytes(b []byte) {
	if b == nil {
		return
	}
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}

