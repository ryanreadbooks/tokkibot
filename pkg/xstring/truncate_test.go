package xstring

import "testing"

func TestTruncateText(t *testing.T) {
	text := "Hello, world!"
	t.Log(Truncate(text, 5))

	text = "你好吗"
	t.Log(Truncate(text, 2))

	text = "你好吗，世界"
	t.Log(Truncate(text, 5))

	// korean word
	text = "안녕하세요"
	t.Log(Truncate(text, 5))

	// japanese word
	text = "こんにちは"
	t.Log(Truncate(text, 5))

	// arabic word with stress mark
	text = "مرحبا, كيف حالك؟"
	t.Log(Truncate(text, 5))

	// russian word with stress mark
	text = "привет, как дела?"
	t.Log(Truncate(text, 5))

	// german word with stress mark
	text = "hallo, wie geht es dir?"
	t.Log(Truncate(text, 5))

	// french word with stress mark
	text = "bonjour à tous"
	t.Log(Truncate(text, 10))

	// spanish word with stress mark
	text = "ho áéíóú"
	t.Log(Truncate(text, 5))
}