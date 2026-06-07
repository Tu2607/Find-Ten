# Project Instructions

## Primary Context
- Always read this `AGENTS.md` first for project context.
- Read `docs/GOAL.md` for project goal, MVP scope, gameplay rules, and future direction.
- Read `docs/ARCHITECTURE.md` for architectural decisions, system boundaries, and design rationale.
- Use the numbered files in `docs/plans/` as the current step-by-step implementation roadmap.
- Read only the relevant `docs/plans/Step-XX.md` file for the current implementation step unless broader plan context is needed.
- Implement the plan step by step. Do not skip ahead unless the user explicitly asks.
- Before starting a new implementation step, confirm the step with the user when practical.
- Coding should not begin on a planned step until the user explicitly approves that step.
- ALWAYS ASK THE DEVELOPERS FOR PERMISSIONS BEFORE MAKING ANY EDITS.
- Do not rewrite unrelated files.
- Do not introduce work outside the current plan step unless the user asks.
- Always go for the simplest working solution first.
- Stick closely to the agreed plan and architecture. If implementation reveals a possible improvement, shortcut, pitfall, or design deviation, raise it to the user before making the change.
- Older completed steps remain as historical records, not rewritten as if the new design had always existed.

## Development Workflow
- Follow the goal document for product behavior and scope.
- Follow the architecture document for design decisions.
- Follow the current plan step file for implementation order and acceptance criteria.
- Each step should compile and pass tests before moving to the next.
- Use table-driven tests for game rules.
- Run `gofmt` on changed Go files.
- Run `go test ./...` after implementation steps.

## Safety
- If `AGENTS.md`, `docs/GOAL.md`, `docs/ARCHITECTURE.md`, and the current `docs/plans/Step-XX.md` conflict, stop and ask the user before proceeding.
- Treat `docs/GOAL.md` as the source of truth for product intent, `docs/ARCHITECTURE.md` as the source of truth for design decisions, and `docs/plans/` as the source of truth for implementation order.
- ALWAYS ASK THE USER WHEN SOMETHING IS UNCLEAR OR NEED VERIFICATION.
