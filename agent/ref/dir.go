package ref

import (
	"crypto/rand"
	"fmt"
	"math/big"
	mrand "math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ryanreadbooks/tokkibot/config"
)

var regChecker = regexp.MustCompile(`^@refs/[a-zA-Z0-9]+$`)

const (
	refDir = "refs"
	Prefix = "@refs/"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateShortID(length int) string {
	b := make([]byte, length)
	max := big.NewInt(int64(len(charset)))
	var err error

	for i := range b {
		var num *big.Int
		num, err = rand.Int(rand.Reader, max)
		if err != nil {
			break
		}
		b[i] = charset[num.Int64()]
	}

	// fallback
	if err != nil {
		for idx := range length {
			b[idx] = charset[mrand.Intn(len(charset))]
		}
	}

	return string(b)
}

// name: without @refs/ prefix
func realRefFilename(name string) string {
	return filepath.Join(config.GetWorkspaceDir(), refDir, name)
}

func Fullpath(ref string) (string, error) {
	if !regChecker.MatchString(ref) {
		return "", fmt.Errorf("invalid ref name format")
	}

	name, ok := strings.CutPrefix(ref, Prefix)
	if ok {
		return realRefFilename(name), nil
	}

	return "", fmt.Errorf("ref %s not found", ref)
}

func GetRandomName() string {
	return generateShortID(10)
}

// Save content to ref file, and return the result ref filename.
//
// Returned ref filename example: @refs/xxx
func Save(content string) (string, error) {
	name := GetRandomName()
	fullpath := realRefFilename(name)
	err := os.MkdirAll(filepath.Dir(fullpath), 0755)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(fullpath, []byte(content), 0644)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%s", Prefix, name), nil
}
