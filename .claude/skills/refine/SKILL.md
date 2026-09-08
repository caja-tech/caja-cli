---
name: refine
description: Runs a Fowler-style refactor-then-test loop (up to 3 rounds) over the just-implemented, uncommitted changes in caja-cli — refactor-agent simplifies the code, test-agent guards against regressions and fills test gaps. Invoke explicitly with /refine right after an implementation's own tests pass.
disable-model-invocation: true
---

Runs refactor-agent and test-agent in an alternating loop over the currently uncommitted diff, for up to 3 rounds, so refactoring happens continuously in small steps (Fowler's discipline) instead of piling up into one big cleanup later.

## Preconditions

Before starting, confirm there IS an uncommitted diff to work on (`git status`) and that it's currently green: `go build ./...` and `go test ./...` both pass. If either fails, stop and tell the user to get back to green before running `/refine` — this skill improves working code, it doesn't debug broken code.

## The loop (max 3 rounds)

For `round` in 1..3:

1. **Refactor**: invoke the Agent tool with `subagent_type: "refactor-agent"`. Give it a one-paragraph description of what was just implemented (pulled from the conversation) so it can judge intentional vs. accidental complexity. Wait for its completion notification — do not poll or guess its result.
2. Read refactor-agent's report.
   - If it changed nothing ("no refactoring opportunities found") **and** this is not round 1, treat refactoring as converged — skip to a final test pass (step 3) and stop the loop after reporting, even if round < 3.
3. **Test**: invoke the Agent tool with `subagent_type: "test-agent"`, noting what refactor-agent just changed (if anything). Wait for its completion notification.
4. Read test-agent's report.
   - **If it reports a regression**: stop the loop immediately. Surface the exact failure to the user (package, test, expected vs. actual, refactor-agent's best guess at cause) and wait for direction — do not launch another round automatically.
   - If it added new test cases, note them for the final summary.
   - If the suite is green with no coverage gaps **and** refactor-agent also found nothing this round: converged — stop after this round even if round < 3.
5. If neither convergence condition hit and `round < 3`, continue to the next round. After round 3, stop regardless.

## Final report

Once the loop ends (convergence, round 3, or a halted regression), print a consolidated summary:
- Per round: what refactor-agent changed (or "nothing"), what test-agent added (or "nothing"), and the suite status.
- Final `go test ./...` status.
- If halted on a regression: say so clearly and what's needed from the user next.

## Notes

- This skill only ever looks at uncommitted changes — commit your work first if you want `/refine` to leave a clean "before" state you can diff against afterward.
- refactor-agent and test-agent both have file-write access and act directly on the code — review the diff afterward (`git diff`) before committing, same as any other agent-authored change.
