package skill

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const (
	SkillMd = "SKILL.md"
)

// Skill can be placed in system workspace/skills as system built-in skills.
// Or skills can be placed in current working directory as project-specific skills.

var namePattern = regexp.MustCompile("^[a-z0-9]+(-[a-z0-9]+)*$")

func validateSkillName(name string) bool {
	l := len(name)
	return namePattern.MatchString(name) && l >= 1 && l <= 64
}

func validateSkillDescription(description string) bool {
	l := len(description)
	return l >= 1 && l <= 1024
}

func validateCompatibility(compatibility string) bool {
	l := len(compatibility)
	return l >= 0 && l <= 500
}

type SkillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
}

// <skill>
// <name>%s</name>
// <description>%s</description>
// </skill>
const tmpl = "<skill>\n\t<name>%s</name>\n\t<description>%s</description>\n</skill>"

func (m *SkillFrontmatter) AsPrompt() string {
	if m != nil {
		return fmt.Sprintf(tmpl, m.Name, m.Description)
	}

	return ""
}

// See: https://agentskills.io/specification for more details.
func (m *SkillFrontmatter) validate(skillName string) error {
	if !validateSkillName(m.Name) {
		return fmt.Errorf("invalid skill name")
	}
	if m.Name != skillName {
		return fmt.Errorf("skill name mismatch")
	}
	if !validateSkillDescription(m.Description) {
		return fmt.Errorf("invalid skill description")
	}
	if !validateCompatibility(m.Compatibility) {
		return fmt.Errorf("invalid skill compatibility")
	}

	return nil
}

type Skill struct {
	// the content of the skill file after the frontmatter in SKILL.md
	content []byte

	// path of the skill
	rawPath string

	Frontmatter SkillFrontmatter

	// lazy loaded
	Refs   []*Refs // references or assets or other resources of the skill
	Script *Script
}

func (s *Skill) Name() string {
	return s.Frontmatter.Name
}

func (s *Skill) Description() string {
	return s.Frontmatter.Description
}

func (s *Skill) Metadata() map[string]string {
	return s.Frontmatter.Metadata
}

func (s *Skill) Validate(name string) error {
	return s.Frontmatter.validate(name)
}

func (s *Skill) LoadRefs(refName string) (*Refs, error) {
	refPath := filepath.Join(s.rawPath, refName) // usaully references/ref1.md or assets/asset1.md
	content, err := os.ReadFile(refPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read reference %s: %w", refName, err)
	}

	return &Refs{
		Path:    refPath,
		Content: string(content),
	}, nil
}

// command should be a valid shell command with arguments
func (s *Skill) ExecuteScript(command string) (string, error) {
	if s.Script == nil {
		s.Script = &Script{Path: s.rawPath}
	}

	return s.Script.Execute(command)
}

// returns xml format prompt for the skill
// prompt only includes frontmatter of the skill
func (s *Skill) AsPrompt() string {
	return s.Frontmatter.AsPrompt()
}

func (s *Skill) Content() string {
	return string(s.content)
}

func SkillsAsPrompt(skills []*Skill) string {
	type promptSkill struct {
		Name        string `xml:"name"`
		Description string `xml:"description"`
	}

	type availableSkills struct {
		XMLName xml.Name       `xml:"available_skills"`
		Skills  []*promptSkill `xml:"skill"`
	}

	ps := make([]*promptSkill, 0, len(skills))
	for _, skill := range skills {
		ps = append(ps, &promptSkill{
			Name:        skill.Name(),
			Description: skill.Description(),
		})
	}

	avs := availableSkills{
		Skills: ps,
	}

	b, _ := xml.MarshalIndent(avs, "", "  ")
	return string(b)
}
