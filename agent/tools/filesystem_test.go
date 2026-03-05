package tools

import (
	"testing"

	"github.com/ryanreadbooks/tokkibot/component/tool"
)

func TestReadFile(t *testing.T) {
	output, err := ReadFile([]string{""}).Invoke(t.Context(),
	 tool.InvokeMeta{
		Channel: "test",
		ChatId:  "test",
	}, `{"path": "./filesystem.go", "offset": 89, "limit": 10}`)
	t.Log(err)
	t.Log(output)
}
