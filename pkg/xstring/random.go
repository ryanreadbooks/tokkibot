package xstring

import (
	"crypto/rand"
	"math/big"
	mrand "math/rand"
)

const (
	charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	alpha   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type randomStringOption struct {
	startsWithAlphabet bool
}

type randomStringOptionFunc func(*randomStringOption)

func WithStartsWithAlphabet(startsWithAlpha bool) randomStringOptionFunc {
	return func(o *randomStringOption) {
		o.startsWithAlphabet = startsWithAlpha
	}
}

func RandomString(length int, opts ...randomStringOptionFunc) string {
	opt := &randomStringOption{
		startsWithAlphabet: false,
	}
	for _, fn := range opts {
		fn(opt)
	}
	if length <= 0 {
		return ""
	}

	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}

	if opt.startsWithAlphabet {
		b[0] = alpha[mrand.Intn(len(alpha))]
	}

	return string(b)
}
