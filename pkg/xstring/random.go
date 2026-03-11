package xstring

import (
	"crypto/rand"
	"math/big"
	mrand "math/rand"
)

const (
	charset      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charsetLower = "abcdefghijklmnopqrstuvwxyz0123456789"
	alpha        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type randomStringOption struct {
	startsWithAlphabet bool
	lowercaseOnly      bool
}

type randomStringOptionFunc func(*randomStringOption)

func WithStartsWithAlphabet(startsWithAlpha bool) randomStringOptionFunc {
	return func(o *randomStringOption) {
		o.startsWithAlphabet = startsWithAlpha
	}
}

func WithLowercaseOnly(lowercaseOnly bool) randomStringOptionFunc {
	return func(o *randomStringOption) {
		o.lowercaseOnly = lowercaseOnly
	}
}

func RandomString(length int, opts ...randomStringOptionFunc) string {
	opt := &randomStringOption{
		startsWithAlphabet: false,
		lowercaseOnly:      false,
	}
	for _, fn := range opts {
		fn(opt)
	}
	if length <= 0 {
		return ""
	}

	chars := charset
	if opt.lowercaseOnly {
		chars = charsetLower
	}

	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}

	if opt.startsWithAlphabet {
		b[0] = alpha[mrand.Intn(len(alpha))]
	}

	return string(b)
}
