# Migrations

goose, SQL, forward-only, **one concern per file**, sequential zero-padded names.

The full inventory — all 22 files, what each creates, and which phase adds it —
is in [`../docs/plan/20-data-model.md`](../docs/plan/20-data-model.md) §13. The
reasoning behind every constraint and index is in the same document, along with
a register of 19 deliberate deviations from the spec sketch.

Read that register before changing a table.

    make migrate        # up
    make migrate-down   # down one
    make migrate-redo   # up -> reset -> up, which is what CI runs

Every file needs a working `-- +goose Down`. `CREATE INDEX CONCURRENTLY` cannot
run inside a transaction, so any file using it needs `-- +goose NO TRANSACTION`.
None of `00001`–`00022` does: they create empty tables, where a plain
`CREATE INDEX` is instant.
