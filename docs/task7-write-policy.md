# Task7 write support

Task6 and Task7 are versions of history-event payloads. The same task UUID can
have a Task7 creation followed by a Task6 modification. Migrating a writer
changes the envelopes it sends; it does not require rewriting an account's
existing tasks or history.

## Supported scope

The CLI sends Task7 for supported task, project, and heading creation and task
modification operations. Tag4, Area3, ChecklistItem3, and Tombstone2 operations
keep their existing formats. The SDK also accepts explicit `ItemKindTask7`
envelopes within the validated scope.

For compatibility, `ItemKindTask` remains Task6. Existing SDK callers continue
to emit the kind they requested. The readers retain support for both versions;
do not change their shared Task6 discriminator to migrate a writer.

- Creation uses the verified complete payload, including null modification
  date and recurrence configuration, and empty recurrence links.
- Project and heading creation is limited to Anytime in this first scope;
  ordinary task creation supports the verified Inbox/Anytime/Someday and
  corresponding date-based scheduling encodings.
- Updates are sparse: omitted properties remain omitted. Supported operations
  include full-note replacement, ordinary scheduling, relationships, tags,
  completion/reopening, and recoverable trash/restoration.
- Direct note-delta writes, recurrence configuration, unsupported properties,
  and Task7 permanent-deletion events are outside this first write contract.

Area and project moves explicitly clear incompatible parent relationships.
An edit cannot combine area with project or heading; project plus heading is
supported. This avoids silently choosing between conflicting destinations or
leaving a task attached to its previous container.

Task7 modifications require an existing, nonrecurring target. Before posting,
`History.Write` reads raw history from index zero and classifies the targeted
UUIDs. A single scan covers every Task7 update in the batch. This avoids
relying on the public `Task` projection, which does not retain every recurrence
field. Unknown/missing targets and recurring templates or instances fail
before any operation in that write request is posted.

This also rejects older or sparse histories without explicit values for all
three recurrence markers (`rr`, `rp`, `rt`), including tasks created by the
legacy sparse SDK example. Such a task may be ordinary, but the new checker
does not infer that from incomplete evidence. CLI updates therefore have a
compatibility limitation for these records; there is no automatic Task6
fallback. Existing explicit Task6 SDK calls keep their previous behavior.

This preflight adds network reads and work proportional to the history size
for update requests. It uses a fixed checked history head as the commit
ancestor; it does not silently retry a failed commit. Existing Task6 writes
keep their prior behavior and do not gain this recurrence protection.

A controlled stale-ancestor test wrote one title update, then submitted a
second update to the same UUID and field using the preceding head. The server
returned HTTP 409 for the second request and history contained only the first
event. This verifies rejection in that case; broader concurrent/offline
convergence remains outside the tested scope.

## Explicit SDK update

```go
update := things.TaskActionItem{
    Item: things.Item{
        UUID: taskUUID,
        Kind: things.ItemKindTask7,
        Action: things.ItemActionModified,
    },
    P: things.TaskActionItemPayload{
        Title: things.String("Updated title"),
        ModificationDate: things.Time(time.Now()),
    },
}
if err := history.Write(update); err != nil {
    return err
}
```

Do not serialize a projected task back as a complete replacement: unmodeled
properties such as modern repeater data cannot be reconstructed from it.
Use the CLI's complete creation payload or an equivalent validated full
envelope for Task7 creation; its required nulls differ from the legacy SDK's
sparse `TaskActionItemPayload` encoding.

## Mixed-version evidence

In the disposable account, the current Task6 CLI performed a title-only edit
on an ordinary Task7 task and on a native Task7 repeating template. Both writes
were persisted as Task6 modifications containing only `tt` and `md`, on the
original UUIDs. Things applied the titles and preserved the checked notes,
scheduling, status, and recurrence columns. The template's stored recurrence
rule remained byte-identical.

The native weekly-after-completion control used a non-null `rr` object with
`rrv=4`, `tp=1`, `fu=256`, and `fa=1`; `rp` was null. The app created a template
and a separate instance linked through `rt`. Completing the instance in Things
emitted a Task7 `ss`/`sp`/`md` update on the instance and an `acrd`/`tir` update
on its template. The app then showed September 13 as the next occurrence.

That confirms title-only mixed-version compatibility for these cases. It
does not establish recurring lifecycle support: changing an envelope's kind
does not synthesize the companion template updates. Non-null modern `rp`,
other recurrence modes, and concurrent/offline convergence remain unverified.

See [the live verification report](task7-write-verification.md) for the tested
ordinary-operation matrix and its limits.
