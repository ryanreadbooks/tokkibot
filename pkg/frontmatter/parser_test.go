package frontmatter

import (
	"bufio"
	"bytes"
	"testing"
)

func TestParser(t *testing.T) {
	reader := &Parser{
		src: bufio.NewReader(bytes.NewReader([]byte("---\nname: test\n---"))),
		dst: bytes.NewBuffer([]byte{}),
	}

	matter, err := reader.parse(&yamlMarker{})
	if err != nil {
		t.Fatalf("failed to parse frontmatter: %v", err)
	}

	t.Log(string(matter))
}

func TestParseYaml(t *testing.T) {
	var dst struct {
		Name        string         `yaml:"name"`
		Description string         `yaml:"description"`
		Metadata    map[string]any `yaml:"metadata"`
	}

	testSrc := `---
name: test
description: test description
metadata:
 key: value
 author: test author
 age: 19
---

# Title helloworld

## -_-_-wow

1. Do something
2. Check something

* - (-)`
	err := ParseYaml([]byte(testSrc), &dst)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	t.Log(dst.Name)
	t.Log(dst.Description)
	t.Log(dst.Metadata)

	t.Log("--------------------------------")

	rest, err := ParseGetYaml([]byte(testSrc), &dst)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	t.Log(string(rest))
	t.Log(dst.Name)
	t.Log(dst.Description)
	t.Log(dst.Metadata)
}

func TestParse2(t *testing.T) {
	text := `---
name: tavily
description: AI-optimized web search via Tavily API. Returns concise, relevant results for AI agents.
homepage: https://tavily.com
metadata: {"clawdbot":{"emoji":"🔍","requires":{"bins":["node"],"env":["TAVILY_API_KEY"]},"primaryEnv":"TAVILY_API_KEY"}}
---

# Tavily Search`

	var dst struct {
		Name        string         `yaml:"name"`
		Description string         `yaml:"description"`
		Metadata    map[string]any `yaml:"metadata"`
	}

	err := ParseYaml([]byte(text), &dst)
	if err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}

	t.Log(dst.Name)
	t.Log(dst.Description)
	t.Log(dst.Metadata)
}
