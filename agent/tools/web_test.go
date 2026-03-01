package tools

import (
	"testing"
)

func TestWebFetch(t *testing.T) {
	output, err := WebFetch().Invoke(t.Context(), `{"url": "https://www.bing.com"}`)
	t.Log(err)
	t.Log(output)
}
