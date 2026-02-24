package agent

import (
	"github.com/ryanreadbooks/tokkibot/agent/context/session"
	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/llm/estimator"
	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

func (a *Agent) RetrieveMessageHistory(channel, chatId string) (
	[]session.LogItem, error,
) {
	history, err := a.contextMgr.GetMessageHistory(
		channel, chatId,
	)
	if err != nil {
		return nil, err
	}

	return history, nil
}

func (a *Agent) AvailableSkills() []*skill.Skill {
	return a.skillLoader.Skills()
}

func (a *Agent) GetSystemPrompt() string {
	return a.contextMgr.GetSystemPrompt()
}

func (a *Agent) InitSession(channel, chatId string) error {
	return a.contextMgr.InitSession(channel, chatId)
}

func (a *Agent) GetCurrentContextTokens(channel, chatId string) int64 {
	est := estimator.RoughEstimator{}
	a.cachedReqsMu.RLock()
	if req, ok := a.cachedReqs[channel+":"+chatId]; ok {
		a.cachedReqsMu.RUnlock()
		tokens, err := est.Estimate(a.c.RootCtx, req)
		if err != nil {
			return 0
		}

		return int64(tokens)
	}
	a.cachedReqsMu.RUnlock()

	// If no cached request, build a minimal request without triggering compaction
	msgList, err := a.contextMgr.GetMessageContext(channel, chatId)
	if err != nil {
		return 0
	}

	// Create a temporary request for estimation only (without calling buildLLMMessageRequest)
	fakeReq := schema.NewRequest(a.c.Model, msgList)
	fakeReq.Tools = a.buildLLMToolParams()

	tokens, err := est.Estimate(a.c.RootCtx, fakeReq)
	if err != nil {
		return 0
	}

	return int64(tokens)
}
