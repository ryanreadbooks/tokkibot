package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/ryanreadbooks/tokkibot/config"
	"github.com/ryanreadbooks/tokkibot/pkg/frontmatter"
	"github.com/ryanreadbooks/tokkibot/pkg/xmap"
)

var ErrSkillNotFound = fmt.Errorf("skill not found")

type Loader struct {
	sysMu        sync.RWMutex
	systemSkills map[string]*Skill

	projMu     sync.RWMutex
	projSkills map[string]*Skill
}

func NewLoader() *Loader {
	l := &Loader{
		systemSkills: make(map[string]*Skill),
		projSkills:   make(map[string]*Skill),
	}

	return l
}

func (l *Loader) Init() error {
	projSkillDir := filepath.Join(config.GetProjectDir(), ".tokkibot", "skills")
	if err := l.collectSkills(projSkillDir, false); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to collect project skills: %w", err)
		}
		// do nothing
	}

	systemSkillDir := filepath.Join(config.GetWorkspaceDir(), "skills")
	if err := l.collectSkills(systemSkillDir, true); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to collect system skills: %w", err)
		}
		// do nothing
	}

	return nil
}

func (l *Loader) getSkill(name string, fromSystem bool) (*Skill, error) {
	if fromSystem {
		l.sysMu.RLock()
		defer l.sysMu.RUnlock()
		sysSkill, ok := l.systemSkills[name]
		if ok {
			return sysSkill, nil
		}

		return nil, ErrSkillNotFound
	}

	l.projMu.RLock()
	defer l.projMu.RUnlock()
	projSkill, ok := l.projSkills[name]
	if ok {
		return projSkill, nil
	}

	return nil, ErrSkillNotFound
}

func (l *Loader) addSkill(skill *Skill, toSystem bool) {
	if toSystem {
		l.sysMu.Lock()
		defer l.sysMu.Unlock()
		l.systemSkills[skill.Name()] = skill
	} else {
		l.projMu.Lock()
		defer l.projMu.Unlock()
		l.projSkills[skill.Name()] = skill
	}
}

func (l *Loader) Get(name string) (*Skill, error) {
	skill, err := l.getSkill(name, false)
	if err == nil {
		return skill, nil
	}

	skill, err = l.getSkill(name, true)
	if err == nil {
		return skill, nil
	}

	return nil, err
}

func (l *Loader) collectSkills(dir string, forSystem bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() { // only directory is considered as skill directory
			skill, err := l.load(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}

			l.addSkill(skill, forSystem)
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

	skillName := stat.Name()
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

	if err := skill.Validate(skillName); err != nil {
		return nil, err
	}

	skill.content = rest
	skill.rawPath = path

	return &skill, nil
}

func (l *Loader) Skills() []*Skill {
	skills := make(map[string]*Skill, len(l.systemSkills)+len(l.projSkills))
	// if project skill has to same name as system skill, system skill will be overwritten
	l.sysMu.RLock()
	defer l.sysMu.RUnlock()
	for _, skill := range l.systemSkills {
		skills[skill.Name()] = skill
	}
	l.projMu.RLock()
	defer l.projMu.RUnlock()
	for _, skill := range l.projSkills {
		skills[skill.Name()] = skill
	}

	// sort the retain stability
	result := xmap.Values(skills)
	sort.Slice(result, func(i int, j int) bool {
		return result[i].Name() < result[j].Name()
	})

	return result
}
