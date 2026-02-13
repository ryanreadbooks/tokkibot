package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/ryanreadbooks/tokkibot/component/skill"
	"github.com/ryanreadbooks/tokkibot/component/tool"
)

type UseSkillAction string

const (
	UseSkillActionActivate UseSkillAction = "activate"
	UseSkillLoad           UseSkillAction = "load"
	UseSkillActionScript   UseSkillAction = "script"
)

// Expected input:
//
// Load rest of the content from SKILL.md file
//
//	{"name": "skillName", "action": "activate"}
//
// Load specified references or assets or other resources given by args
//
//	{"name": "skillName", "action": "load", "args": "ref1,asset1"}
//
// Execute specified script given by args
//
//	{"name": "skillName", "action": "script", "args": "python script.py --arg1 arg1 --arg2 arg2"}
type UseSkillInput struct {
	Name   string         `json:"name"           jsonschema:"description=The name of the skill to load"`
	Action UseSkillAction `json:"action"         jsonschema:"description=The action to perform on a skill, one of: activate, load, script"`
	Args   string         `json:"args,omitempty" jsonschema:"description=The arguments passed to skill action."`
}

// Skill-using tool.
func UseSkill(loader *skill.Loader) tool.Invoker {
	return tool.NewInvoker(tool.Info{
		Name:        "use_skill",
		Description: "Use skill with the given name using action and args",
	}, func(ctx context.Context, input *UseSkillInput) (string, error) {
		s, err := loader.Get(input.Name)
		if err != nil {
			return "", err
		}

		switch input.Action {
		case UseSkillActionActivate:
			return s.Content(), nil
		case UseSkillLoad:
			refs := strings.Split(input.Args, ",")
			refManual := strings.Builder{}
			for _, ref := range refs {
				refManual.WriteString(ref + "\n\n")
				refContent, err := s.LoadRefs(ref)
				if err != nil {
					refManual.WriteString(err.Error() + "\n")
				} else {
					refManual.WriteString(refContent.Content + "\n")
				}

				refManual.WriteString("---\n")
			}

			return refManual.String(), nil
		case UseSkillActionScript:
			return s.ExecuteScript(input.Args)
		}

		return "", fmt.Errorf("invalid skill action: %s", input.Action)
	})
}
