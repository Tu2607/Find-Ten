# Project Instructions

## Primary Context
- Always read this `AGENTS.md` first for project context.
- Read `docs/GOAL.md` for project goal, MVP scope, gameplay rules, and future direction.
- Read `docs/ARCHITECTURE.md` for architectural decisions, system boundaries, and design rationale.
- Use `docs/PLAN.md` as the current step-by-step implementation roadmap.
- Implement the plan step by step. Do not skip ahead unless the user explicitly asks.
- Before starting a new implementation step, confirm the step with the user when practical.

## Development Workflow
- Follow the goal document for product behavior and scope.
- Follow the architecture document for design decisions.
- Follow the plan document for implementation order and acceptance criteria.
- Use table-driven tests for game rules.
- Run `gofmt` on changed Go files.
- Run `go test ./...` after implementation steps.

## Safety
- Do not rewrite unrelated files.
- Do not introduce work outside the current plan step unless the user asks.
- Stick closely to the agreed plan and architecture. If implementation reveals a possible improvement, shortcut, pitfall, or design deviation, raise it to the user before making the change.
- If `AGENTS.md`, `docs/GOAL.md`, `docs/ARCHITECTURE.md`, and `docs/PLAN.md` conflict, stop and ask the user before proceeding.
- Treat `docs/GOAL.md` as the source of truth for product intent, `docs/ARCHITECTURE.md` as the source of truth for design decisions, and `docs/PLAN.md` as the source of truth for implementation order.
- ALWAYS ASK THE USER WHEN SOMETHING IS UNCLEAR OR NEED VERIFICATION.
