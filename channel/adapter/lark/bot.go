package lark

import (
	"context"
	"fmt"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

// https://open.feishu.cn/document/client-docs/bot-v3/obtain-bot-info
type BotInfo struct {
	ActivateStatus int      `json:"activate_status"`
	AppName        string   `json:"app_name"`
	AvatarUrl      string   `json:"avatar_url"`
	IpWhiteList    []string `json:"ip_white_list"`
	OpenId         string   `json:"open_id"`
}

type GetBotInfoResp struct {
	*larkcore.ApiResp `json:"-"`
	larkcore.CodeError
	Bot *BotInfo `json:"bot"`
}

func (resp *GetBotInfoResp) Success() bool {
	return resp.Code == 0
}

func (a *LarkAdapter) GetBotInfo(ctx context.Context) (*BotInfo, error) {
	apiResp, err := a.cli.Get(ctx, "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return nil, err
	}

	resp := &GetBotInfoResp{ApiResp: apiResp}
	err = apiResp.JSONUnmarshalBody(resp, &larkcore.Config{Serializable: &larkcore.DefaultSerialization{}})
	if err != nil {
		return nil, err
	}

	if !resp.Success() {
		return nil, fmt.Errorf("failed to get bot info: %s", resp.ErrorResp())
	}

	return resp.Bot, nil
}

func (a *LarkAdapter) GetBotOpenId(ctx context.Context) (string, error) {
	a.botOpenIdMu.RLock()
	botOpenId := a.botOpenId
	a.botOpenIdMu.RUnlock()

	if botOpenId == "" {
		// get from remote api 
		botInfo, err := a.GetBotInfo(ctx)
		if err != nil {
			return "", err
		}
		botOpenId = botInfo.OpenId
		a.botOpenIdMu.Lock()
		a.botOpenId = botOpenId
		a.botOpenIdMu.Unlock()
	}

	return a.botOpenId, nil
}
