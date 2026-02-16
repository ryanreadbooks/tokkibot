package tools

import (
	"testing"
)

func TestReadFile(t *testing.T) {
	output, err := ReadFile([]string{""}).Invoke(t.Context(),
	 `{"path": "./filesystem.go", "offset": 89, "limit": 10}`)
	t.Log(err)
	t.Log(output)
}
