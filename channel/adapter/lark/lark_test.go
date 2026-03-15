package lark

import (
	"context"
	"os"
	"testing"
)

func TestHandleTextMessage(t *testing.T) {
	content := `{"text":"Hello, @_user_1 @_user_2 @_user_3"}`
	a := LarkAdapter{}
	content, mentionedUsers, err := a.handleTextMessage(content)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(content)
	t.Log(mentionedUsers)
}

func TestGetBotInfo(t *testing.T) {
	a := NewAdapter(LarkConfig{
		AppId:     os.Getenv("FEISHU_APP_ID"),
		AppSecret: os.Getenv("FEISHU_APP_SECRET"),
	})
	botInfo, err := a.GetBotInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Log(botInfo)
}
