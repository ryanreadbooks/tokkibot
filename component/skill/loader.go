package skill

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/pkg/frontmatter"
	"github.com/ryanreadbooks/tokkibot/pkg/xmap"
)

var ErrSkillNotFound = fmt.Errorf("skill not found")

// skillScope represents which level a skill belongs to.
// Higher scope overrides lower scope when names conflict.
type skillScope int

const (
	scopeGlobal  skillScope = iota // ~/.tokkibot/skills (shared by all agents)
	scopeAgent                     // ~/tokkibot/workspace[-{name}]/skills (per-agent)
	scopeProject                   // ./.tokkibot/skills (per-project, highest priority)
)

type Loader struct {
	mu     sync.RWMutex
	skills map[skillScope]map[string]*Skill
}

func NewLoader() *Loader {
	return &Loader{
		skills: map[skillScope]map[string]*Skill{
			scopeGlobal:  {},
			scopeAgent:   {},
			scopeProject: {},
		},
	}
}

// Init loads skills from three sources (low → high priority):
//  1. Global: ~/.tokkibot/skills (shared by all agents)
//  2. Agent:  agentWorkspace/skills (per-agent)
//  3. Project: ./.tokkibot/skills (per-project)
func (l *Loader) Init(agentWorkspace string) error {
	globalSkillDir := filepath.Join(config.GetHomeDir(), "skills")
	if err := l.collectSkills(globalSkillDir, scopeGlobal); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to collect global skills: %w", err)
		}
	}

	agentSkillDir := filepath.Join(agentWorkspace, "skills")
	if err := l.collectSkills(agentSkillDir, scopeAgent); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to collect agent skills: %w", err)
		}
	}

	projSkillDir := filepath.Join(config.GetProjectDir(), ".tokkibot", "skills")
	if err := l.collectSkills(projSkillDir, scopeProject); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to collect project skills: %w", err)
		}
	}

	return nil
}

func (l *Loader) Get(name string) (*Skill, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// search from highest priority to lowest
	for _, scope := range []skillScope{scopeProject, scopeAgent, scopeGlobal} {
		if s, ok := l.skills[scope][name]; ok {
			return s, nil
		}
	}

	return nil, ErrSkillNotFound
}

func (l *Loader) collectSkills(dir string, scope skillScope) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skill, err := l.load(filepath.Join(dir, entry.Name()))
			if err != nil {
				slog.Warn("failed to load skill", "path", filepath.Join(dir, entry.Name()), "error", err)
				continue
			}

			l.mu.Lock()
			l.skills[scope][skill.Name()] = skill
			l.mu.Unlock()
		}
	}

	return nil
}

// Load single skill from given path.
//
// The given path must be a valid skill directory containing at minimum a `SKILL.md` file.
func (l *Loader) load(path string) (*Skill, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		return nil, fmt.Errorf("path %s is not a skill directory", path)
	}

	mdPath := filepath.Join(path, SkillMd)
	mdStat, err := os.Stat(mdPath)
	if err != nil {
		return nil, err
	}

	if !mdStat.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", mdPath)
	}

	skillFileName := stat.Name()
	mdContent, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, err
	}

	var skill Skill
	// rest the the content after the frontmatter
	rest, err := frontmatter.ParseGetYaml(mdContent, &skill.Frontmatter)
	if err != nil {
		return nil, err
	}

	if err := skill.Validate(skillFileName); err != nil {
		return nil, err
	}

	skill.content = rest
	skill.rawPath = path

	return &skill, nil
}

func (l *Loader) Skills() []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	merged := make(map[string]*Skill)
	// apply from lowest to highest priority so higher scopes overwrite
	for _, scope := range []skillScope{scopeGlobal, scopeAgent, scopeProject} {
		for _, s := range l.skills[scope] {
			merged[s.Name()] = s
		}
	}

	result := xmap.Values(merged)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})

	return result
}
