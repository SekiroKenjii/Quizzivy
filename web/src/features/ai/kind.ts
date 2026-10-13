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

/** AiField is one setup field of a panel, drawn by `AiPanelField`. */
export type AiField =
  | {
      kind: "source";
      label: string;
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
  | { kind: "info"; text: string };

/** AiRunResult is what a run answers: the items to review and what it cost. `notice` is an info line above them. */
export interface AiRunResult<I> {
  items: I[];
  usage: AiUsage;
  notice?: string;
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
 * AiPrimary is a panel's primary button after a run: its label and the write
 * it asks the host for, which answers the toast. `available` says whether the
 * answer holds anything it can save; without it, or when it says no, the
 * button is "Done".
 */
export interface AiPrimary<I, C> {
  icon: (env: AiEnv<C>) => LucideIcon;
  available?: (items: readonly I[]) => boolean;
  label: (selected: readonly I[], env: AiEnv<C>) => string;
  apply: (selected: readonly I[], env: AiEnv<C>) => Promise<AiDone>;
}

/**
 * AiKind is one AI panel: its header, its setup fields and their checks, its
 * run, how each item it answers is drawn and what its primary button saves.
 * `C` is what the host page gives the panel, including every write: a kind
 * calls only `/teacher/ai/*` itself.
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
  problem: (values: F) => string | null;
  run: (values: F, env: AiEnv<C>, signal: AbortSignal) => Promise<AiRunResult<I>>;
  card: (item: I, env: AiCardEnv<C>) => AiCard;
  summary: (items: readonly I[], selected: number, t: TFunction) => string;
  primary: AiPrimary<I, C> | null;
}
