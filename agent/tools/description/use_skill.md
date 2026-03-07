Interact with skills to extend capabilities. Skills are defined in SKILL.md files located in workspace/skills (system) or project directory (project-specific).

## Actions

### `activate`
Load the full SKILL.md content (instructions, guidelines, examples). Use this first to understand how to use the skill.

```json
{"name": "skill-name", "action": "activate"}
```

### `load`
Load reference files or assets from the skill directory. Useful for loading templates, examples, or documentation.

```json
{"name": "skill-name", "action": "load", "args": "references/guide.md"}
{"name": "skill-name", "action": "load", "args": "assets/template.json,references/api.md"}
```
- `args`: Comma-separated paths relative to skill directory

### `script`
Execute a script/command in the skill's directory. The working directory is set to the skill's root path.

```json
{"name": "skill-name", "action": "script", "args": "python generate.py --input data.json"}
{"name": "skill-name", "action": "script", "args": "node build.js --env production"}
```
- `args`: Full command with arguments (executed with 5-minute timeout)
- Output is truncated to 15000 chars if too long

## Typical Workflow
1. `activate` — Read skill instructions to understand usage
2. `load` — Load any needed references or assets
3. `script` — Execute scripts if the skill provides automation

## Notes
- Skill names use kebab-case: `my-skill-name`
- Available skills are listed in system prompt under `<available_skills>`
