import type { TFunction } from "i18next";
import type { LucideIcon } from "lucide-react";
import type { FormOption } from "@/components/shared/form/fields/types";
import type { AiLanguage, AiUsage } from "./api";

/**
 * AiWriteError is a host's refusal of a write a panel asked for, before
 * anything was sent; the panel shows its message as it is.
 */
export class AiWriteError extends Error {
  override readonly name = "AiWriteError";
}

/**
 * AiPartialWriteError is a write that landed for some of the items it was
 * given and was refused for the rest: the panel drops `saved` from its list
 * and shows the message above what is left.
 */
export class AiPartialWriteError extends AiWriteError {
  readonly saved: readonly unknown[];

  constructor(message: string, saved: readonly unknown[]) {
    super(message);
    this.saved = saved;
  }
}

/** AiTone is the colour pair of a pill on an item card. */
export type AiTone = "muted" | "accent" | "danger" | "warning" | "info" | "success";

/**
 * AiCardAction is a button on an item card. `run` does the work and answers
 * the toast to show, or null for none; `drop` removes the card afterwards;
 * `href` makes the action a link instead.
 */
export interface AiCardAction {
  label: string;
  primary?: boolean;
  run?: () => Promise<string | null> | string | null;
  drop?: boolean;
  href?: string;
}

/** AiCard is what one item card draws, in the deck's order from the pills down. */
export interface AiCard {
  selectable: boolean;
  tags: readonly { label: string; tone: AiTone }[];
  meta?: string;
  title: string;
  body?: string;
  options?: readonly { text: string; correct: boolean }[];
  answer?: string;
  diff?: {
    beforeLabel: string;
    before: string;
    afterLabel: string;
    after: string;
    strike: boolean;
  };
  chips?: readonly string[];
  note?: { label: string; text: string };
  actions?: readonly AiCardAction[];
}

/** AiSourceChip is one ready source above a source field: a passage, or none. */
export interface AiSourceChip {
  key: string;
  label: string;
  icon: LucideIcon;
  active: boolean;
  pick: () => void;
}

/** AiOption is one choice of a radio card group or a destination list. */
export interface AiOption {
  value: string;
  label: string;
  sub?: string;
  italic?: boolean;
  badge?: string;
}

/**
 * AiPlanRow is one section of a plan: its count from 0 to `max`, or why it
 * cannot take one, and once it has a count, its style, types and level.
 */
export interface AiPlanRow {
  key: string;
  title: string;
  italic: boolean;
  meta: string;
  count: string;
  onCount: (value: string) => void;
  blocked?: string;
  unit?: string;
  mode?: {
    label: string;
    options: readonly FormOption[];
    value: string;
    onChange: (value: string) => void;
    calc: string;
  };
  types: {
    label: string;
    hint: string;
    options: readonly FormOption[];
    value: readonly string[];
    onChange: (value: readonly string[]) => void;
  };
  level: {
    label: string;
    hint: string;
    options: readonly FormOption[];
    value: string;
    onChange: (value: string) => void;
  };
}

/** AiField is one setup field of a panel, drawn by `AiPanelField`. */
export type AiField =
  | {
      kind: "source";
      label: string;
      optional?: string;
      value: string;
      placeholder: string;
      rows: number;
      sources: readonly AiSourceChip[];
      onChange: (value: string) => void;
    }
  | {
      kind: "chips";
      label: string;
      options: readonly FormOption[];
      value: readonly string[];
      onChange: (value: readonly string[]) => void;
    }
  | {
      kind: "seg";
      label: string;
      sub?: string;
      hideLabel?: boolean;
      options: readonly FormOption[];
      value: string;
      onChange: (value: string) => void;
    }
  | {
      kind: "select";
      label: string;
      options: readonly FormOption[];
      value: string;
      onChange: (value: string) => void;
    }
  | {
      kind: "toggle";
      text: string;
      sub?: string;
      value: boolean;
      onChange: (value: boolean) => void;
    }
  | {
      kind: "radios";
      label: string;
      options: readonly AiOption[];
      value: string;
      onChange: (value: string) => void;
    }
  | {
      kind: "destination";
      label: string;
      change: string;
      changeLabel: string;
      options: readonly AiOption[];
      value: string;
      onChange: (value: string) => void;
      toggle: {
        text: string;
        sub: string;
        value: boolean;
        onChange: (value: boolean) => void;
      };
    }
  | {
      kind: "stepper";
      label: string;
      hint: string;
      fewer: string;
      more: string;
      value: string;
      min: number;
      max: number;
      onChange: (value: string) => void;
    }
  | {
      kind: "plan";
      label: string;
      sub: string;
      head: { section: string; count: string };
      fewer: string;
      more: string;
      max: number;
      rows: readonly AiPlanRow[];
      total: string;
      over: readonly string[];
    }
  | {
      kind: "alert";
      text: string;
      choose?: {
        label: string;
        options: readonly FormOption[];
        value: string;
        onChange: (value: string) => void;
      };
    }
  | { kind: "info"; text: string };

/**
 * AiPart is one part of a run of several calls, as its running stage draws
 * it: writing, done with `count` items, or failed.
 */
export interface AiPart {
  key: string;
  title: string;
  meta: string;
  state: "running" | "done" | "failed";
  count: number;
}

/**
 * AiFailedPart is a part of a run that answered nothing: its group's key, the
 * reason when the server gave one, `max`, the most a retry may spend in
 * credits, and `retry`, which runs that part alone.
 */
export interface AiFailedPart<I> {
  key: string;
  message?: string;
  max: number;
  retry: (signal: AbortSignal) => Promise<AiRunResult<I>>;
}

/**
 * AiGroup is the heading an item is drawn under after a run of several parts,
 * in `order`, with an optional sub-heading and its note.
 */
export interface AiGroup {
  key: string;
  title: string;
  order: number;
  sub?: { key: string; title: string; note: string };
}

/**
 * AiRunResult is what a run answers: the items to review and what it cost.
 * `notice` is an info line above them; `groups` heads every planned part, in
 * order, and `failed` holds the parts that answered nothing.
 */
export interface AiRunResult<I> {
  items: I[];
  usage: AiUsage;
  notice?: string;
  groups?: readonly AiGroup[];
  failed?: readonly AiFailedPart<I>[];
}

/**
 * AiEnv is what every part of a kind can read: the strings, the language the
 * assistant writes in, whether its drafts are tagged, and the host's context.
 */
export interface AiEnv<C> {
  t: TFunction;
  language: AiLanguage;
  tagDrafts: boolean;
  context: C;
}

/** AiCardEnv is an AiEnv that can also close the panel, for an action that leaves it. */
export interface AiCardEnv<C> extends AiEnv<C> {
  close: () => void;
}

/** AiDone is the toast a primary answers: its sentence, and a link to follow when there is one. */
export type AiDone =
  string | { message: string; action: { label: string; href: string } };

/**
 * AiPrimary is a panel's primary button after a run: its icon and label,
 * which may read every item the run answered, and the write it asks the host
 * for, which answers the toast. `available` says whether the answer holds
 * anything it can save; without it, or when it says no, the button is "Done".
 * `blocked` disables it while the answer, or the picks in `selected`, cannot
 * be saved as they stand.
 */
export interface AiPrimary<I, C, F = unknown> {
  icon: (env: AiEnv<C>, items: readonly I[], values: F) => LucideIcon;
  available?: (items: readonly I[]) => boolean;
  blocked?: (
    items: readonly I[],
    env: AiEnv<C>,
    values: F,
    selected: readonly I[],
  ) => boolean;
  label: (
    selected: readonly I[],
    env: AiEnv<C>,
    items: readonly I[],
    values: F,
  ) => string;
  apply: (selected: readonly I[], env: AiEnv<C>, values: F) => Promise<AiDone>;
}

/**
 * AiCeiling is the most a run can be billed before it starts, and whether it
 * is a plan of several calls, for the line beside the run button.
 */
export interface AiCeiling {
  max: number;
  plan: boolean;
}

/**
 * AiKind is one AI panel: its header, its setup fields and their checks, its
 * run, how each item it answers is drawn and what its primary button saves.
 * `stale` says an item answered earlier no longer holds for the page as it is
 * now, and the panel leaves it out of the list and every count. `C` is what
 * the host page gives the panel, including every write: a kind calls only
 * `/teacher/ai/*` itself. The optional parts let a kind name its steps, its
 * run button and its empty line from the values, show its credit ceiling,
 * report the parts of a run of several calls through `report`, head its items
 * by `group`, and add fields to its results (`resultFields`), whose values the
 * primary reads.
 */
export interface AiKind<F, I, C> {
  icon: LucideIcon;
  titleKey: string;
  subtitleKey: string;
  runLabelKey: string;
  stepKeys: readonly string[];
  emptyKey: string;
  initial: (context: C) => F;
  fields: (
    values: F,
    set: (patch: Partial<F>) => void,
    context: C,
    t: TFunction,
  ) => AiField[];
  problem: (values: F, context: C) => string | null;
  steps?: (values: F, context: C, t: TFunction) => readonly string[];
  runLabel?: (values: F, context: C, t: TFunction) => string;
  empty?: (values: F, t: TFunction) => string;
  ceiling?: (values: F, context: C) => AiCeiling;
  run: (
    values: F,
    env: AiEnv<C>,
    signal: AbortSignal,
    report: (parts: readonly AiPart[]) => void,
  ) => Promise<AiRunResult<I>>;
  card: (item: I, env: AiCardEnv<C>) => AiCard;
  group?: (item: I) => AiGroup;
  stale?: (item: I, context: C) => boolean;
  resultFields?: (
    values: F,
    set: (patch: Partial<F>) => void,
    items: readonly I[],
    context: C,
    t: TFunction,
  ) => AiField[];
  summary: (items: readonly I[], selected: number, t: TFunction, values: F) => string;
  primary: AiPrimary<I, C, F> | null;
}
