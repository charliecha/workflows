# Workflows Project

## Available Skills

The following skills are installed via `npx skills add smallnest/goal-workflow`.
Invoke them by typing `/skill-name` in the chat.

- `/prd` — Generate a Product Requirements Document
- `/prd-to-spec` — Convert PRD into a technical specification
- `/to-issues` — Decompose PRD/SPEC into GitHub Issues
- `/review-it` — Automated code review before committing
- `/ship-it` — Commit, PR, merge, close issue
- `/refactor` — Expert refactoring (Fowler's catalog)
- `/modern-go` — Modernize Go code idioms
- `/insight-diagram` — Generate UML/architecture diagrams
- `/code-to-spec` — Reverse-engineer spec from existing code
- `/humanize-it` — Humanize generated text
- `/note-it` — Take notes
- `/listenhub-tts` — Text to speech

## Skill Files

Skills are located in `.agents/skills/`. To use a skill, mention its name
or type `/skill-name` — Claude will load and execute the corresponding SKILL.md.
