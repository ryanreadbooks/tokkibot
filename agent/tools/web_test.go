package tools

import (
	"testing"

	"github.com/ryanreadbooks/tokkibot/component/tool"
)

func TestWebFetch(t *testing.T) {
	output, err := WebFetch().Invoke(t.Context(),
		tool.InvokeMeta{Channel: "test", ChatId: "test"},
		`{"url": "https://www.bing.com"}`)
	t.Log(err)
	t.Log(output)
}
