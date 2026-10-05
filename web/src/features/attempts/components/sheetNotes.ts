/** SheetNoteDraft exposes one immutable attempt's editable value and save/recovery state. */
export type SheetNoteDraft = Readonly<{
  value: string;
  saved: string | null;
  pending: boolean;
  error: unknown;
}>;

type Entry = {
  value: string;
  saved: string | null;
  pending: Promise<boolean> | null;
  error: unknown;
};

function noteValue(value: string): string | null {
  return value.trim() === "" ? null : value;
}

/** SheetNoteController serializes blur and departure saves per attempt without replacing newer edits or dirty refetch drafts. */
export class SheetNoteController {
  private readonly entries = new Map<string, Entry>();
  private readonly listeners = new Set<() => void>();
  private revision = 0;
  private readonly save: (id: string, value: string | null) => Promise<unknown>;

  constructor(save: (id: string, value: string | null) => Promise<unknown>) {
    this.save = save;
  }

  readonly subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  readonly snapshot = () => this.revision;

  private tell() {
    this.revision++;
    for (const listener of this.listeners) listener();
  }

  accept(id: string, saved: string | null) {
    const entry = this.entries.get(id);
    if (
      entry !== undefined &&
      (entry.pending !== null ||
        entry.error !== null ||
        noteValue(entry.value) !== entry.saved ||
        entry.saved === saved)
    )
      return;
    this.entries.set(id, { value: saved ?? "", saved, pending: null, error: null });
    this.tell();
  }

  get(id: string): SheetNoteDraft | undefined {
    const entry = this.entries.get(id);
    return entry === undefined
      ? undefined
      : {
          value: entry.value,
          saved: entry.saved,
          pending: entry.pending !== null,
          error: entry.error,
        };
  }

  change(id: string, value: string) {
    const entry = this.entries.get(id);
    if (!entry) return;
    entry.value = value;
    this.tell();
  }

  get unsettled(): boolean {
    return [...this.entries.values()].some(
      (entry) =>
        entry.pending !== null ||
        entry.error !== null ||
        noteValue(entry.value) !== entry.saved,
    );
  }

  async flush(id: string): Promise<boolean> {
    const entry = this.entries.get(id);
    if (!entry) return true;
    if (entry.pending !== null) {
      if (!(await entry.pending)) return false;
      return this.flush(id);
    }
    if (entry.value.length > 2000) {
      entry.error = new Error("Note exceeds its length limit");
      this.tell();
      return false;
    }
    const value = noteValue(entry.value);
    if (value === entry.saved) {
      entry.error = null;
      this.tell();
      return true;
    }
    entry.error = null;
    const operation = Promise.resolve()
      .then(() => this.save(id, value))
      .then(
        () => {
          entry.saved = value;
          entry.error = null;
          return true;
        },
        (error: unknown) => {
          entry.error = error;
          return false;
        },
      )
      .finally(() => {
        entry.pending = null;
        this.tell();
      });
    entry.pending = operation;
    this.tell();
    if (!(await operation)) return false;
    return this.flush(id);
  }

  async flushAll(): Promise<boolean> {
    for (const id of this.entries.keys()) {
      if (!(await this.flush(id))) return false;
    }
    return true;
  }

  async discardAll(): Promise<void> {
    for (const entry of this.entries.values()) {
      while (entry.pending !== null) await entry.pending;
      entry.value = entry.saved ?? "";
      entry.error = null;
    }
    this.tell();
  }
}
