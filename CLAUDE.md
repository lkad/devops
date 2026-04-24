# DevOps Toolkit

## Project Overview

Go-based internal DevOps platform for managing infrastructure, CI/CD pipelines, logs, alerts, and physical hosts.

## Key Files

- [ARCHITECTURE.md](ARCHITECTURE.md) — Backend architecture, data models, API structure
- [DESIGN.md](DESIGN.md) — Frontend design system (colors, typography, spacing)
- [PRD.md](PRD.md) — Product requirements and design
- [TODOS.md](TODOS.md) — Task tracking and status

## Skill routing

When the user's request matches an available skill, ALWAYS invoke it using the Skill tool as your FIRST action. Do NOT answer directly, do NOT use other tools first.

Key routing rules:
- Product ideas, "is this worth building", brainstorming → invoke office-hours
- Bugs, errors, "why is this broken", 500 errors → invoke investigate
- Ship, deploy, push, create PR → invoke ship
- QA, test the site, find bugs → invoke qa
- Code review, check my diff → invoke review
- Update docs after shipping → invoke document-release
- Weekly retro → invoke retro
- Design system, brand → invoke design-consultation
- Visual audit, design polish → invoke design-review
- Architecture review → invoke plan-eng-review
- Save progress, checkpoint, resume → invoke checkpoint
- Code quality, health check → invoke health
