# DevOps Toolkit

## Project Overview

Go-based internal DevOps platform for managing infrastructure, CI/CD pipelines, logs, alerts, and physical hosts.

## Documentation

**📖 必读:** [DOCUMENT_INDEX.md](DOCUMENT_INDEX.md) — 文档索引和阅读顺序

权威来源:
- [PRD.md](PRD.md) — 产品需求 v2.1
- [openspec/specs/](openspec/specs/) — 形式化技术规格
- [REQUIREMENTS.md](REQUIREMENTS.md) — 后端技术规格
- [DESIGN.md](DESIGN.md) — 前端设计系统
- [docs/CONFLICTS.md](docs/CONFLICTS.md) — 文档冲突清单

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
