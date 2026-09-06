# Future scheduled task fix

## Scope

Correct CLI Task6 payloads so a scheduled local calendar date in the future is
classified as Upcoming (`st=2`), while today and past dates remain
Today/Anytime (`st=1`). Keep `--when` authoritative when it is supplied and
preserve the existing UTC-midnight encoding for `sr` and `tir`.

This change is stacked on PR #30 and reuses its shared task-option validation.

## Implementation plan

1. Add deterministic regression coverage for future, today, and past scheduled
   dates using a fixed-day helper, including explicit `--when` precedence.
2. Cover create, edit, and batch-create payloads, plus project, heading, and
   area relationships, while asserting the existing Task6 fields stay intact.
3. Centralize the date-to-schedule classification and use it from create and
   update payload builders. Prevent relationship auto-Anytime behavior from
   replacing either an explicit `--when` or an explicit scheduled date.
4. Retain PR #30's rejection of malformed or valueless scheduled options at
   direct create, edit, and batch-create boundaries. Reuse the shared validator
   instead of adding a second validation implementation.
5. Document the interaction between `--scheduled` and `--when`, then run the
   focused CLI tests followed by the repository's normal build and lint checks.

## Constraints

- No new dependencies or cloud writes.
- Do not change the 34-field create payload or the UTC-midnight civil-date
  representation.
- Preserve the safety rule that projects and headings cannot be Inbox items.
