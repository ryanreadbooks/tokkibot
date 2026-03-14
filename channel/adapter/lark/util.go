package lark

import imv1 "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

// derefStr safely dereferences a string pointer, returning empty string if nil
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// messageTarget encapsulates the recipient of a lark message (user or chat).
type messageTarget struct {
	idType string
	id     string
}

func userTarget(openId string) messageTarget {
	return messageTarget{idType: imv1.ReceiveIdTypeOpenId, id: openId}
}

func chatTarget(chatId string) messageTarget {
	return messageTarget{idType: imv1.ReceiveIdTypeChatId, id: chatId}
}
