# Online Skill Marketplace Integration

VibeCoding is compatible with existing skill marketplaces (SkillHub / ClawHub). Skill packages published on these platforms can be used directly in VibeCoding.

| Platform | URL | Region |
|----------|-----|--------|
| **SkillHub** | [https://skillhub.cn](https://skillhub.cn/) | China |
| **ClawHub** | [https://clawhub.ai](https://clawhub.ai/) | International |

> **Note:** VibeCoding does not have a built-in skill marketplace, but uses the standard
> skill directory format (`SKILL.md`) that is fully compatible with SkillHub / ClawHub
> packages. Skills downloaded from these platforms work out of the box — just drop them
> into your skills directory.

This guide covers:

1. [Installing Skills from Marketplaces](#installing-skills-from-marketplaces) — three steps
2. [Skill Format Compatibility](#skill-format-compatibility) — standard format details
3. [Local Skill System](#local-skill-system) — built-in features
4. [Cron Foundation](#cron-foundation) — scheduled task infrastructure

---

## Installing Skills from Marketplaces

Installing skills from SkillHub / ClawHub takes three steps:

### 1. Download the Skill Package

Download the skill package from the marketplace (typically a directory or archive containing `SKILL.md`).

### 2. Extract to Skills Directory

```bash
# Global install (available to all projects)
# Linux/macOS:
unzip go-expert.zip -d ~/.vibecoding/skills/
# Windows:
Expand-Archive go-expert.zip -DestinationPath "$env:APPDATA\vibecoding\skills\"

# Project-level install (current project only)
unzip go-expert.zip -d .skills/
```

### 3. Verify Installation

```
> /skills
Loaded 3 skills:
  - go-expert (global)        ← just installed
  - coding-standards (global)
  - project-conventions (project)
```

That's it. The skill is automatically loaded and injected into the system prompt.

---

## Skill Format Compatibility

VibeCoding's skill format is fully compatible with the SkillHub / ClawHub standard:

```
skill-name/
├── SKILL.md              # Required: skill definition
└── references/           # Optional: on-demand reference files
    ├── api-guide.md
    └── examples.md
```

### SKILL.md Standard Format

```markdown
# Skill Name

Short description.

## Rules

- Rule 1
- Rule 2

## Examples

...
```

### Reference Files

Skills can include reference files under a `references/` directory, loaded on demand via the `skill_ref` tool:

```
> skill_ref(skill="go-expert", ref="references/api-guide.md")
→ Returns the content of api-guide.md
```

This allows skills to include extensive reference material without consuming system prompt space.

---

## Local Skill System

In addition to marketplace downloads, you can create local skills directly.

### Skill Directories

| Type | Location | Scope |
|------|----------|-------|
| Global | `~/.vibecoding/skills/` (Linux/macOS) or `%APPDATA%\vibecoding\skills\` (Windows) | All projects |
| Project | `.skills/` (project root) | Current project, overrides global |

### Creating a Skill

```bash
mkdir -p ~/.vibecoding/skills/go-expert
cat > ~/.vibecoding/skills/go-expert/SKILL.md << 'EOF'
# Go Expert

Expert-level Go coding standards.

## Rules

- Use `gofmt` for formatting
- Follow Effective Go guidelines
- Return errors; do not panic
- Use `fmt.Errorf` with `%w` for wrapping

## Testing

- Write table-driven tests
- Use `t.Run` for subtests
- Aim for >80% coverage
EOF
```

### Using Skills

```
> /skills
Loaded 2 skills:
  - go-expert (global)
  - project-conventions (project)

> /skill:go-expert
Loaded skill: go-expert
```

### Configuration

Configure the global skills directory in `settings.json`:

```json
{
  "skillsDir": "~/.vibecoding/skills"
}
```

Project skills load automatically from `.skills/` without extra configuration.

---

## Cron Foundation

VibeCoding has an internal cron infrastructure (`internal/cron` package) and TUI command entry points. The cron store persists jobs to `~/.vibecoding/cron.json` and the scheduler checks for due jobs on a 30-second interval.

### `/cron` TUI Commands

Requires multi-agent mode (`--multi-agent` or Ctrl+P to toggle):

```
> /cron add <description>      — Add a scheduled task
> /cron list                   — List scheduled tasks
> /cron enable <id>            — Enable a task
> /cron disable <id>           — Disable a task
> /cron remove <id>            — Remove a task
> /cron run <id>               — Run a task now
```

### Cron Job Data Model

| Field | Description |
|-------|-------------|
| `id` | Unique job ID (e.g. `cron-1716883200`) |
| `name` | Short task description |
| `prompt` | Task prompt for sub-agent |
| `schedule` | 5-field cron expression |
| `mode` | `agent` or `yolo` |
| `enabled` | Whether the job is active |
| `last_run` | Timestamp of last execution |
| `next_run` | Computed next execution time |
| `run_count` | Total executions |
| `last_status` | `success`, `failed`, or `running` |

### Scheduler Architecture

```
Scheduler loop (every 30s)
    │
    ├── List all enabled jobs from store
    │
    ├── Check each job: is it due?
    │   ├── Never run before → due
    │   ├── NextRun has passed → due
    │   └── Last run > 1 hour ago → due (fallback)
    │
    └── Due jobs → spawn sub-agent
              │
              ├── Mark job as "running"
              ├── Create agent via AgentManager
              ├── Run agent with job prompt
              ├── Collect result
              └── Update job status (success/failed)
```

---

## Related Documents

- [Skills System](skills.md) — Local skills format and management
- [Configuration](configuration.md) — Full settings reference
- [Security](security.md) — Sandbox and approval controls
