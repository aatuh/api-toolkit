# Backlog

Project: api-toolkit

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Harden countrycodes dataset loading [x]

Description: Make `contrib/countrycodes` safe to reload and consistent with the CSV contract it already documents.

### Ticket E1-T1 - Preserve the existing dataset on load failures [x]

Description: Refactor `LoadCSV` so it only swaps package-global country data after a full successful parse, and add regression tests proving a failed reload does not wipe previously valid data.

Implementation rules:

- implement the ticket in the smallest sensible step
- run targeted unit tests for the touched package after completing the ticket
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `contrib/countrycodes/countrycodes.go`
- add a new `contrib/countrycodes/countrycodes_test.go`

### Ticket E1-T2 - Honor documented CSV headers by name [x]

Description: Resolve the `name` and `alpha-2` columns from the CSV header row instead of assuming fixed positions, and add unit tests that prove the loader accepts valid reordered input.

Implementation rules:

- implement the ticket in the smallest sensible step
- run targeted unit tests for the touched package after completing the ticket
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `contrib/countrycodes/countrycodes.go`
- extend `contrib/countrycodes/countrycodes_test.go`
