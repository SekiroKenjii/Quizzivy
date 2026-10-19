import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import {
  Check,
  Layers,
  LoaderCircle,
  RefreshCw,
  Sparkles,
  type LucideIcon,
} from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { IconTile } from "@/components/shared/IconTile";
import { Sheet } from "@/components/shared/Sheet";
import { Button } from "@/components/ui/button";
import { failureMessage } from "@/lib/api/errors";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { credits } from "../credits";
import type { AiUsage } from "../api";
import {
  aiFailureOf,
  refreshAfterAiFailure,
  settleAiUsage,
  type AiFailure,
} from "../outcome";
import {
  AiPartialWriteError,
  AiWriteError,
  type AiCard,
  type AiCardAction,
  type AiCeiling,
  type AiEnv,
  type AiFailedPart,
  type AiGroup,
  type AiKind,
  type AiPart,
  type AiRunResult,
} from "../kind";
import { checkKind, type CheckContext } from "../kinds/check";
import { cleanupKind, type CleanupContext } from "../kinds/cleanup";
import { generateKind, type GenerateContext } from "../kinds/generate";
import { variantKind, type VariantContext } from "../kinds/variant";
import {
  AI_BLOCK_KEYS,
  type AiBlock,
  useAiBlock,
  useAiLanguage,
  useAiLeft,
  useAiTagsDrafts,
} from "../useAiAvailable";
import { useFinishedToast } from "../useFinishedToast";
import { AiCreditNote } from "./AiCreditNote";
import { AiItemCard } from "./AiItemCard";
import { AiPanelField } from "./AiPanelField";
import { AiRunParts } from "./AiRunParts";
import { AiRunSteps } from "./AiRunSteps";

/** AiPanelRequest names the panel to open and what its host page gives it. */
export type AiPanelRequest =
  | { kind: "generate"; context: GenerateContext }
  | { kind: "check"; context: CheckContext }
  | { kind: "variant"; context: VariantContext }
  | { kind: "cleanup"; context: CleanupContext };

type Keyed<I> = { key: string; item: I };

type Failed<I> = AiFailedPart<I> & { retrying: boolean };

type Stage<I> =
  | { name: "setup"; failure: AiFailure | null }
  | { name: "running"; parts: readonly AiPart[] | null }
  | {
      name: "done";
      items: Keyed<I>[];
      selected: ReadonlySet<string>;
      usage: AiUsage;
      notice: string | undefined;
      groups: readonly AiGroup[];
      failed: readonly Failed<I>[];
    };

type Drawn<I> = { key: string; item: I; card: AiCard };

const FOOTER_BUTTON = "h-9 max-[767px]:h-11 max-[767px]:flex-[1_1_120px]";
const SMALL_BUTTON = "max-[767px]:h-10";

let nextKey = 0;

function keyed<I>(items: readonly I[]): Keyed<I>[] {
  return items.map((item) => {
    nextKey += 1;
    return { key: `k${nextKey}`, item };
  });
}

function grouped<I>(
  cards: readonly Drawn<I>[],
  groups: readonly AiGroup[],
  group: (item: I) => AiGroup,
  failed: readonly Failed<I>[],
) {
  const order = [...groups].sort((a, b) => a.order - b.order);
  return order
    .map((head) => {
      const own = cards.filter(({ item }) => group(item).key === head.key);
      const subs: { key: string; head?: AiGroup["sub"]; cards: Drawn<I>[] }[] = [];
      for (const entry of own) {
        const sub = group(entry.item).sub;
        const key = sub?.key ?? "";
        let found = subs.find((candidate) => candidate.key === key);
        if (!found) {
          found = { key, ...(sub ? { head: sub } : {}), cards: [] };
          subs.push(found);
        }
        found.cards.push(entry);
      }
      return {
        head,
        cards: own,
        subs,
        failed: failed.find((part) => part.key === head.key),
      };
    })
    .filter((entry) => entry.cards.length > 0 || entry.failed !== undefined);
}

function writeFailure(cause: unknown, fallback: string): string {
  return cause instanceof AiWriteError
    ? cause.message
    : failureMessage(cause, fallback);
}

/**
 * AiPanel is the deck's AI drawer (V-14) on the `Sheet`, 560px from the
 * right: a kind's setup fields, its running steps, and the suggestions it
 * answered, with the footer's credit note and buttons. It closes on Esc,
 * keeps focus inside and returns it to what opened it (DG-36). The panel and
 * its kinds call only `/teacher/ai/*`; every write a primary or a card makes
 * is the host page's own, handed in through the request's context. No
 * sample output is ever drawn: a failed run is a callout (DG-201). The
 * session is the panel's state, not the drawer's: while `open` is false it
 * draws nothing and keeps its fields, a run in flight and the suggestions a
 * run answered, so closing never discards them. A run that settles while the
 * drawer is closed says so in a toast, whose action calls `onReopen`; while
 * another drawer is `aside` showing, it says so when that one closes. Done, a
 * primary's write and Start over end the session, and so does unmounting,
 * which also aborts a run that has not answered; a host keeps the panel as
 * long as its page (`AiPanelStack`).
 */
export default function AiPanel({
  request,
  open = true,
  aside = false,
  onClose,
  onReopen,
}: Readonly<{
  request: AiPanelRequest;
  open?: boolean;
  aside?: boolean;
  onClose: () => void;
  onReopen?: () => void;
}>) {
  switch (request.kind) {
    case "generate":
      return (
        <AiKindPanel
          kind={generateKind}
          context={request.context}
          open={open}
          aside={aside}
          onClose={onClose}
          onReopen={onReopen}
        />
      );
    case "check":
      return (
        <AiKindPanel
          kind={checkKind}
          context={request.context}
          open={open}
          aside={aside}
          onClose={onClose}
          onReopen={onReopen}
        />
      );
    case "variant":
      return (
        <AiKindPanel
          kind={variantKind}
          context={request.context}
          open={open}
          aside={aside}
          onClose={onClose}
          onReopen={onReopen}
        />
      );
    case "cleanup":
      return (
        <AiKindPanel
          kind={cleanupKind}
          context={request.context}
          open={open}
          aside={aside}
          onClose={onClose}
          onReopen={onReopen}
        />
      );
  }
}

type Done<I> = Extract<Stage<I>, { name: "done" }>;

function onDone<I>(update: (current: Done<I>) => Stage<I>) {
  return (current: Stage<I>) => (current.name === "done" ? update(current) : current);
}

function marked<I>(
  current: Done<I>,
  key: string,
  on: boolean,
  message: string | undefined,
): Stage<I> {
  return {
    ...current,
    failed: current.failed.map((part) =>
      part.key === key
        ? { ...part, retrying: on, ...(message === undefined ? {} : { message }) }
        : part,
    ),
  };
}

function selecting<I>(
  current: Done<I>,
  keys: Iterable<string>,
  on: boolean,
): ReadonlySet<string> {
  const selected = new Set(current.selected);
  for (const key of keys) {
    if (on) selected.add(key);
    else selected.delete(key);
  }
  return selected;
}

function retried<I>(
  current: Done<I>,
  key: string,
  added: readonly Keyed<I>[],
  selectable: (item: I) => boolean,
  result: AiRunResult<I>,
): Stage<I> {
  return {
    ...current,
    items: [...current.items, ...added],
    selected: selecting(
      current,
      added.filter(({ item }) => selectable(item)).map(({ key: added }) => added),
      true,
    ),
    usage: {
      credits: current.usage.credits + result.usage.credits,
      left: result.usage.left,
    },
    notice: current.notice ?? result.notice,
    failed: [
      ...current.failed.filter((entry) => entry.key !== key),
      ...(result.failed ?? []).map((entry) => ({ ...entry, retrying: false })),
    ],
  };
}

function blockOf<I>(stage: Stage<I>): AiBlock | null {
  return stage.name === "setup" && stage.failure?.kind === "blocked"
    ? stage.failure.block
    : null;
}

function sectionsOf<I>(
  done: Done<I> | null,
  group: ((item: I) => AiGroup) | undefined,
  cards: readonly Drawn<I>[],
) {
  if (!done || !group || done.groups.length === 0) return null;
  return grouped(cards, done.groups, group, done.failed);
}

function SetupFailure({
  failure,
  retryDisabled,
  onRetry,
}: Readonly<{
  failure: AiFailure | null;
  retryDisabled: boolean;
  onRetry: () => void;
}>) {
  const { t } = useTranslation();
  if (failure?.kind === "unavailable")
    return (
      <Callout
        tone="warning"
        size="md"
        announce
        actions={
          <Button
            variant="outline"
            size="sm"
            className={cn("bg-card", SMALL_BUTTON)}
            disabled={retryDisabled}
            onClick={onRetry}
          >
            {t("ai.tryAgain")}
          </Button>
        }
      >
        {t("ai.unavailable")}
      </Callout>
    );
  if (failure?.kind === "error")
    return (
      <Callout tone="danger" size="md" announce>
        {failure.message}
      </Callout>
    );
  return null;
}

function creditNote({
  t,
  locale,
  used,
  problem,
  stop,
  left,
  ceiling,
  short,
}: {
  t: TFunction;
  locale: Locale;
  used: { credits: number; left: number } | null;
  problem: string | null;
  stop: AiBlock | null;
  left: number | null;
  ceiling: AiCeiling | null;
  short: boolean;
}): { warning: boolean; text: string } | null {
  if (used)
    return {
      warning: false,
      text: t("ai.credits.used", {
        credits: credits(used.credits, locale),
        left: credits(used.left, locale),
      }),
    };
  if (problem !== null) return { warning: true, text: t(problem) };
  if (stop !== null) return { warning: true, text: t(AI_BLOCK_KEYS[stop]) };
  if (left === null) return null;
  if (ceiling === null)
    return {
      warning: false,
      text: t("ai.credits.left", { left: credits(left, locale) }),
    };
  const figures = { max: credits(ceiling.max, locale), left: credits(left, locale) };
  if (short)
    return {
      warning: true,
      text: t(ceiling.plan ? "ai.credits.shortTest" : "ai.credits.shortOne", figures),
    };
  return { warning: false, text: t("ai.credits.max", figures) };
}

function AiKindPanel<F, I, C>({
  kind,
  context,
  open,
  aside,
  onClose,
  onReopen,
}: Readonly<{
  kind: AiKind<F, I, C>;
  context: C;
  open: boolean;
  aside: boolean;
  onClose: () => void;
  onReopen: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const language = useAiLanguage();
  const tagDrafts = useAiTagsDrafts();
  const block = useAiBlock(true);
  const left = useAiLeft();
  const [values, setValues] = useState<F>(() => kind.initial(context));
  const [stage, setStage] = useState<Stage<I>>({ name: "setup", failure: null });
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const run = useRef<AbortController | null>(null);
  const retries = useRef(new Set<AbortController>());
  const announce = useFinishedToast({
    open,
    aside,
    title: t(kind.titleKey),
    onReopen,
  });

  useEffect(() => {
    const pending = retries.current;
    return () => {
      run.current?.abort();
      for (const controller of pending) controller.abort();
    };
  }, []);

  const env: AiEnv<C> = { t, language, tagDrafts, context };
  const set = (patch: Partial<F>) => setValues((current) => ({ ...current, ...patch }));
  const problem = kind.problem(values, context);
  const stop = blockOf(stage) ?? block;
  const ceiling = kind.ceiling?.(values, context) ?? null;
  const short = ceiling !== null && left !== null && ceiling.max > left;

  async function start() {
    if (problem !== null || stop !== null || short) return;
    run.current?.abort();
    const controller = new AbortController();
    run.current = controller;
    setActionError(null);
    setStage({ name: "running", parts: null });
    try {
      const result = await kind.run(values, env, controller.signal, (parts) => {
        if (!controller.signal.aborted) setStage({ name: "running", parts });
      });
      if (controller.signal.aborted) return;
      settleAiUsage(queryClient, result.usage);
      const items = keyed(result.items);
      const card = (item: I) => kind.card(item, { ...env, close: onClose });
      setStage({
        name: "done",
        items,
        selected: new Set(
          items.filter(({ item }) => card(item).selectable).map(({ key }) => key),
        ),
        usage: result.usage,
        notice: result.notice,
        groups: result.groups ?? [],
        failed: (result.failed ?? []).map((part) => ({ ...part, retrying: false })),
      });
      announce("done");
    } catch (cause) {
      if (controller.signal.aborted) return;
      const failure = aiFailureOf(cause, t("ai.failed"));
      refreshAfterAiFailure(queryClient, cause, failure);
      setStage({ name: "setup", failure });
      announce("failed");
    }
  }

  function retrying(key: string, on: boolean, message?: string) {
    setStage(onDone((current) => marked(current, key, on, message)));
  }

  async function retry(part: Failed<I>) {
    const controller = new AbortController();
    retries.current.add(controller);
    retrying(part.key, true);
    try {
      const result = await part.retry(controller.signal);
      if (controller.signal.aborted) return;
      settleAiUsage(queryClient, result.usage);
      const added = keyed(result.items);
      setStage(
        onDone((current) =>
          retried(
            current,
            part.key,
            added,
            (item) => kind.card(item, { ...env, close: onClose }).selectable,
            result,
          ),
        ),
      );
    } catch (cause) {
      if (controller.signal.aborted) return;
      const failure = aiFailureOf(cause, t("ai.failed"));
      refreshAfterAiFailure(queryClient, cause, failure);
      retrying(part.key, false, failure.kind === "error" ? failure.message : undefined);
    } finally {
      retries.current.delete(controller);
    }
  }

  function halt() {
    run.current?.abort();
    setStage({ name: "setup", failure: null });
  }

  function remove(keys: ReadonlySet<string>) {
    setStage(
      onDone((current) => ({
        ...current,
        items: current.items.filter((entry) => !keys.has(entry.key)),
        selected: selecting(current, keys, false),
      })),
    );
  }

  function select(keys: readonly string[], on: boolean) {
    setStage(
      onDone((current) => ({ ...current, selected: selecting(current, keys, on) })),
    );
  }

  async function act(key: string, action: AiCardAction) {
    setActionError(null);
    if (action.run === undefined) {
      if (action.drop) remove(new Set([key]));
      return;
    }
    setBusy(true);
    try {
      const message = await action.run();
      if (message !== null) notify.success(message);
      if (action.drop) remove(new Set([key]));
    } catch (cause) {
      setActionError(writeFailure(cause, t("ai.applyFailed")));
    } finally {
      setBusy(false);
    }
  }

  function conclude() {
    setActionError(null);
    setStage({ name: "setup", failure: null });
    onClose();
  }

  async function apply(selected: readonly Drawn<I>[]) {
    if (kind.primary === null) return;
    setActionError(null);
    setBusy(true);
    try {
      const done = await kind.primary.apply(
        selected.map(({ item }) => item),
        env,
        values,
      );
      if (typeof done === "string") notify.success(done);
      else
        notify.success(done.message, {
          action: {
            label: done.action.label,
            onClick: () => void navigate(done.action.href),
          },
        });
      setBusy(false);
      conclude();
    } catch (cause) {
      if (cause instanceof AiPartialWriteError) {
        const saved = new Set(cause.saved);
        remove(
          new Set(selected.filter(({ item }) => saved.has(item)).map(({ key }) => key)),
        );
      }
      setActionError(writeFailure(cause, t("ai.applyFailed")));
      setBusy(false);
    }
  }

  const done = stage.name === "done" ? stage : null;
  const live =
    done?.items.filter(({ item }) => !(kind.stale?.(item, context) ?? false)) ?? [];
  const cards: Drawn<I>[] = live.map(({ key, item }) => ({
    key,
    item,
    card: kind.card(item, { ...env, close: onClose }),
  }));
  const selectable = cards
    .filter((entry) => entry.card.selectable)
    .map(({ key }) => key);
  const answered = cards.map(({ item }) => item);
  const selectedCards = cards.filter((entry) => done?.selected.has(entry.key));
  const selectedItems = selectedCards.map(({ item }) => item);
  const allSelected =
    selectable.length > 0 && selectable.every((key) => done?.selected.has(key));
  const sections = sectionsOf(done, kind.group, cards);

  const note = creditNote({
    t,
    locale,
    used: done?.usage ?? null,
    problem: stage.name === "setup" ? problem : null,
    stop,
    left,
    ceiling,
    short: stage.name === "setup" && short,
  });

  const primaryShown =
    kind.primary !== null &&
    answered.length > 0 &&
    (kind.primary.available?.(answered) ?? true);

  const footer = (
    <>
      {note === null ? (
        <span className="min-w-0 flex-[1_1_180px]" />
      ) : (
        <AiCreditNote text={note.text} warning={note.warning} />
      )}
      {stage.name === "setup" && (
        <>
          <Button variant="outline" className={FOOTER_BUTTON} onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            className={FOOTER_BUTTON}
            disabled={problem !== null || stop !== null || short}
            onClick={() => void start()}
          >
            <Sparkles aria-hidden="true" />
            {kind.runLabel?.(values, context, t) ?? t(kind.runLabelKey)}
          </Button>
        </>
      )}
      {stage.name === "running" && (
        <Button variant="outline" className={FOOTER_BUTTON} onClick={halt}>
          {t("ai.stop")}
        </Button>
      )}
      {done && (
        <div className="ml-auto flex max-w-full min-w-0 flex-wrap items-center justify-end gap-x-2.5 gap-y-2 max-[767px]:flex-[1_1_100%]">
          <Button
            variant="outline"
            className={FOOTER_BUTTON}
            disabled={busy}
            onClick={() => setStage({ name: "setup", failure: null })}
          >
            {t("ai.startOver")}
          </Button>
          {primaryShown && kind.primary !== null ? (
            <PrimaryButton
              icon={kind.primary.icon(env, answered, values)}
              label={kind.primary.label(selectedItems, env, answered, values)}
              disabled={
                busy ||
                selectedItems.length === 0 ||
                (kind.primary.blocked?.(answered, env, values) ?? false)
              }
              onClick={() => void apply(selectedCards)}
            />
          ) : (
            <PrimaryButton icon={Check} label={t("ai.done")} onClick={conclude} />
          )}
        </div>
      )}
    </>
  );

  function itemCard({ key, card }: Drawn<I>) {
    return (
      <AiItemCard
        key={key}
        card={card}
        selected={done?.selected.has(key) ?? false}
        busy={busy}
        onToggle={() => select([key], !(done?.selected.has(key) ?? false))}
        onAction={(action) => void act(key, action)}
      />
    );
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
      width={560}
      title={t(kind.titleKey)}
      subtitle={t(kind.subtitleKey)}
      leading={
        <IconTile
          icon={kind.icon}
          size={36}
          tone="accent"
          className="rounded-[0.625rem]"
        />
      }
      footer={footer}
    >
      {stage.name === "setup" && (
        <SetupFailure
          failure={stage.failure}
          retryDisabled={problem !== null || stop !== null || short}
          onRetry={() => void start()}
        />
      )}
      {stage.name === "setup" &&
        kind
          .fields(values, set, context, t)
          .map((field, index) => <AiPanelField key={index} field={field} />)}
      {stage.name === "running" &&
        (stage.parts === null ? (
          <AiRunSteps
            steps={
              kind.steps?.(values, context, t) ?? kind.stepKeys.map((key) => t(key))
            }
          />
        ) : (
          <AiRunParts parts={stage.parts} />
        ))}
      {done && (
        <>
          {actionError !== null && (
            <Callout tone="danger" size="md" announce>
              {actionError}
            </Callout>
          )}
          {(kind.resultFields?.(values, set, answered, context, t) ?? []).map(
            (field, index) => (
              <AiPanelField key={index} field={field} />
            ),
          )}
          {done.notice !== undefined && cards.length > 0 && (
            <Callout tone="info" size="md">
              {done.notice}
            </Callout>
          )}
          {cards.length > 0 && (
            <div className="flex min-h-7.5 flex-wrap items-center justify-between gap-x-2.5 gap-y-1">
              <p className="text-muted-fg text-sm text-pretty">
                {kind.summary(answered, selectedItems.length, t, values)}
              </p>
              {selectable.length > 0 && (
                <button
                  type="button"
                  className={cn(
                    "hover:bg-muted text-meta rounded-seg h-7.5 px-2.5 font-medium whitespace-nowrap",
                    SMALL_BUTTON,
                  )}
                  onClick={() => select(selectable, !allSelected)}
                >
                  {allSelected ? t("ai.clearSelection") : t("ai.selectAll")}
                </button>
              )}
            </div>
          )}
          {cards.length === 0 && (sections === null || sections.length === 0) && (
            <p className="text-muted-fg text-ui rounded-xl border border-dashed px-4 py-7 text-center text-pretty">
              {kind.empty?.(values, t) ?? t(kind.emptyKey)}
            </p>
          )}
          {sections === null
            ? cards.map(itemCard)
            : sections.map((section) => (
                <ResultSection
                  key={section.head.key}
                  section={section}
                  selected={done.selected}
                  canRetry={stop === null}
                  left={left}
                  locale={locale}
                  itemCard={itemCard}
                  onSelect={select}
                  onRetry={(failed) => void retry(failed)}
                />
              ))}
        </>
      )}
    </Sheet>
  );
}

type Section<I> = ReturnType<typeof grouped<I>>[number];

function ResultSection<I>({
  section: { head, cards: own, subs, failed },
  selected,
  canRetry,
  left,
  locale,
  itemCard,
  onSelect,
  onRetry,
}: Readonly<{
  section: Section<I>;
  selected: ReadonlySet<string>;
  canRetry: boolean;
  left: number | null;
  locale: Locale;
  itemCard: (drawn: Drawn<I>) => React.ReactNode;
  onSelect: (keys: readonly string[], on: boolean) => void;
  onRetry: (failed: Failed<I>) => void;
}>) {
  const { t } = useTranslation();
  const keys = own.filter((entry) => entry.card.selectable).map(({ key }) => key);
  const chosen = keys.filter((key) => selected.has(key)).length;
  const all = keys.length > 0 && chosen === keys.length;
  return (
    <section aria-label={head.title} className="flex flex-col gap-2.5">
      <div className="bg-card sticky -top-4.5 z-3 -mx-4.5 flex flex-wrap items-center gap-x-2.5 gap-y-1 border-b px-4.5 py-2.5">
        <h3 className="text-ui min-w-0 flex-[1_1_200px] truncate font-semibold">
          {head.title}
        </h3>
        {keys.length > 0 && (
          <>
            <span className="text-muted-fg text-meta whitespace-nowrap">
              {t("ai.generate.sectionSelected", {
                n: chosen,
                total: keys.length,
              })}
            </span>
            <button
              type="button"
              className={cn(
                "hover:bg-muted text-meta rounded-seg h-7.5 px-2.5 font-medium whitespace-nowrap",
                SMALL_BUTTON,
              )}
              onClick={() => onSelect(keys, !all)}
            >
              {all ? t("ai.clearSelection") : t("ai.selectAll")}
            </button>
          </>
        )}
      </div>
      {failed !== undefined &&
        (failed.retrying ? (
          <p
            role="status"
            className="bg-muted flex items-center gap-2 rounded-[0.5rem] px-3 py-2.5 text-sm"
          >
            <LoaderCircle
              aria-hidden="true"
              className="size-3.5 animate-spin motion-reduce:animate-none"
            />
            {t("ai.generate.sectionWriting")}
          </p>
        ) : (
          <Callout
            tone="warning"
            size="md"
            announce
            actions={
              <Button
                variant="outline"
                size="sm"
                className={cn("bg-card", SMALL_BUTTON)}
                disabled={!canRetry || (left !== null && failed.max > left)}
                onClick={() => onRetry(failed)}
              >
                <RefreshCw aria-hidden="true" />
                {t("ai.tryAgain")}
              </Button>
            }
          >
            <span className="flex flex-col">
              <span>{failed.message ?? t("ai.generate.sectionFailed")}</span>
              <span className="text-muted-fg text-meta">
                {t("ai.generate.retryCost", { max: credits(failed.max, locale) })}
              </span>
            </span>
          </Callout>
        ))}
      {subs.map((sub) => (
        <div key={sub.key} className="flex flex-col gap-2.5">
          {sub.head && (
            <p className="text-muted-fg text-meta flex flex-wrap items-center gap-x-1.5 gap-y-0.5">
              <Layers aria-hidden="true" className="size-3.25" />
              <strong className="text-fg font-semibold">{sub.head.title}</strong>
              <span>· {sub.head.note}</span>
            </p>
          )}
          {sub.cards.map(itemCard)}
        </div>
      ))}
    </section>
  );
}

function PrimaryButton({
  icon: Icon,
  label,
  disabled = false,
  onClick,
}: Readonly<{
  icon: LucideIcon;
  label: string;
  disabled?: boolean;
  onClick: () => void;
}>) {
  return (
    <Button
      className={cn(FOOTER_BUTTON, "max-w-full min-w-0 shrink")}
      title={label}
      disabled={disabled}
      onClick={onClick}
    >
      <Icon aria-hidden="true" />
      <span className="min-w-0 truncate">{label}</span>
    </Button>
  );
}
