import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { Check, Sparkles, type LucideIcon } from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { IconTile } from "@/components/shared/IconTile";
import { Sheet } from "@/components/shared/Sheet";
import { Button } from "@/components/ui/button";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { notify } from "@/lib/toast";
import { credits } from "../credits";
import { AI_STATUS_KEY, AI_USAGE_KEY, type AiMonthUsage, type AiUsage } from "../api";
import { AiWriteError, type AiCardAction, type AiEnv, type AiKind } from "../kind";
import { checkKind, type CheckContext } from "../kinds/check";
import { generateKind, type GenerateContext } from "../kinds/generate";
import {
  AI_BLOCK_KEYS,
  useAiBlock,
  useAiLanguage,
  useAiLeft,
  useAiTagsDrafts,
  type AiBlock,
} from "../useAiAvailable";
import { AiCreditNote } from "./AiCreditNote";
import { AiItemCard } from "./AiItemCard";
import { AiPanelField } from "./AiPanelField";
import { AiRunSteps } from "./AiRunSteps";

/** AiPanelRequest names the panel to open and what its host page gives it. */
export type AiPanelRequest =
  | { kind: "generate"; context: GenerateContext }
  | { kind: "check"; context: CheckContext };

type Failure =
  | { kind: "unavailable" }
  | { kind: "blocked"; block: AiBlock }
  | { kind: "error"; message: string };

type Keyed<I> = { key: string; item: I };

type Stage<I> =
  | { name: "setup"; failure: Failure | null }
  | { name: "running" }
  | {
      name: "done";
      items: Keyed<I>[];
      selected: ReadonlySet<string>;
      usage: AiUsage;
      notice: string | undefined;
    };

const FOOTER_BUTTON = "h-9";

function writeFailure(cause: unknown, fallback: string): string {
  return cause instanceof AiWriteError
    ? cause.message
    : failureMessage(cause, fallback);
}

function failureOf(cause: unknown, t: (key: string) => string): Failure {
  if (!(cause instanceof ApiError)) return { kind: "unavailable" };
  if (cause.code === "AI_CREDITS_EXHAUSTED")
    return {
      kind: "blocked",
      block: cause.details?.["scope"] === "pool" ? "pool" : "teacher",
    };
  if (cause.code === "AI_PAUSED") return { kind: "blocked", block: "paused" };
  if (cause.code === "AI_UNAVAILABLE" || cause.status === 0 || cause.status >= 500)
    return { kind: "unavailable" };
  return { kind: "error", message: failureMessage(cause, t("ai.failed")) };
}

/**
 * AiPanel is the deck's AI drawer (V-14) on the `Sheet`, 560px from the
 * right: a kind's setup fields, its running steps, and the suggestions it
 * answered, with the footer's credit note and buttons. It closes on Esc,
 * keeps focus inside and returns it to what opened it (DG-36). The panel and
 * its kinds call only `/teacher/ai/*`; every write a primary or a card makes
 * is the host page's own, handed in through the request's context. No
 * sample output is ever drawn: a failed run is a callout (DG-201).
 */
export default function AiPanel({
  request,
  onClose,
}: Readonly<{ request: AiPanelRequest; onClose: () => void }>) {
  switch (request.kind) {
    case "generate":
      return (
        <AiKindPanel kind={generateKind} context={request.context} onClose={onClose} />
      );
    case "check":
      return (
        <AiKindPanel kind={checkKind} context={request.context} onClose={onClose} />
      );
  }
}

function AiKindPanel<F, I, C>({
  kind,
  context,
  onClose,
}: Readonly<{ kind: AiKind<F, I, C>; context: C; onClose: () => void }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const queryClient = useQueryClient();
  const language = useAiLanguage();
  const tagDrafts = useAiTagsDrafts();
  const block = useAiBlock(true);
  const left = useAiLeft();
  const [values, setValues] = useState<F>(() => kind.initial(context));
  const [stage, setStage] = useState<Stage<I>>({ name: "setup", failure: null });
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const run = useRef<AbortController | null>(null);

  useEffect(() => () => run.current?.abort(), []);

  const env: AiEnv<C> = { t, language, tagDrafts, context };
  const set = (patch: Partial<F>) => setValues((current) => ({ ...current, ...patch }));
  const problem = kind.problem(values);
  const failedBlock =
    stage.name === "setup" && stage.failure?.kind === "blocked"
      ? stage.failure.block
      : null;
  const stop = failedBlock ?? block;

  function settleUsage(usage: AiUsage) {
    queryClient.setQueryData<AiMonthUsage>(AI_USAGE_KEY, (month) =>
      month ? { ...month, used: month.used + usage.credits, left: usage.left } : month,
    );
  }

  async function start() {
    if (problem !== null || stop !== null) return;
    run.current?.abort();
    const controller = new AbortController();
    run.current = controller;
    setActionError(null);
    setStage({ name: "running" });
    try {
      const result = await kind.run(values, env, controller.signal);
      if (controller.signal.aborted) return;
      settleUsage(result.usage);
      const items = result.items.map((item, index) => ({ key: `k${index}`, item }));
      const card = (item: I) => kind.card(item, { ...env, close: onClose });
      setStage({
        name: "done",
        items,
        selected: new Set(
          items.filter(({ item }) => card(item).selectable).map(({ key }) => key),
        ),
        usage: result.usage,
        notice: result.notice,
      });
    } catch (cause) {
      if (controller.signal.aborted) return;
      const failure = failureOf(cause, t);
      if (failure.kind === "blocked")
        void queryClient.invalidateQueries({ queryKey: AI_USAGE_KEY });
      if (cause instanceof ApiError && cause.code === "AI_OFF")
        void queryClient.invalidateQueries({ queryKey: AI_STATUS_KEY });
      setStage({ name: "setup", failure });
    }
  }

  function halt() {
    run.current?.abort();
    setStage({ name: "setup", failure: null });
  }

  function drop(key: string) {
    setStage((current) => {
      if (current.name !== "done") return current;
      const selected = new Set(current.selected);
      selected.delete(key);
      return {
        ...current,
        items: current.items.filter((entry) => entry.key !== key),
        selected,
      };
    });
  }

  function toggle(key: string) {
    setStage((current) => {
      if (current.name !== "done") return current;
      const selected = new Set(current.selected);
      if (selected.has(key)) selected.delete(key);
      else selected.add(key);
      return { ...current, selected };
    });
  }

  async function act(key: string, action: AiCardAction) {
    setActionError(null);
    if (action.run === undefined) {
      if (action.drop) drop(key);
      return;
    }
    setBusy(true);
    try {
      const message = await action.run();
      if (message !== null) notify.success(message);
      if (action.drop) drop(key);
    } catch (cause) {
      setActionError(writeFailure(cause, t("ai.applyFailed")));
    } finally {
      setBusy(false);
    }
  }

  async function apply(selected: readonly I[]) {
    if (kind.primary === null) return;
    setActionError(null);
    setBusy(true);
    try {
      const message = await kind.primary.apply(selected, env);
      notify.success(message);
      onClose();
    } catch (cause) {
      setActionError(writeFailure(cause, t("ai.applyFailed")));
      setBusy(false);
    }
  }

  const done = stage.name === "done" ? stage : null;
  const cards =
    done?.items.map(({ key, item }) => ({
      key,
      item,
      card: kind.card(item, { ...env, close: onClose }),
    })) ?? [];
  const selectable = cards
    .filter((entry) => entry.card.selectable)
    .map(({ key }) => key);
  const selectedItems = cards
    .filter((entry) => done?.selected.has(entry.key))
    .map(({ item }) => item);
  const allSelected =
    selectable.length > 0 && selectable.every((key) => done?.selected.has(key));

  const note = (() => {
    if (done)
      return {
        warning: false,
        text: t("ai.credits.used", {
          credits: credits(done.usage.credits, locale),
          left: credits(done.usage.left, locale),
        }),
      };
    if (problem !== null) return { warning: true, text: t(problem) };
    if (stop !== null) return { warning: true, text: t(AI_BLOCK_KEYS[stop]) };
    if (left !== null)
      return {
        warning: false,
        text: t("ai.credits.left", { left: credits(left, locale) }),
      };
    return null;
  })();

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
            disabled={problem !== null || stop !== null}
            onClick={() => void start()}
          >
            <Sparkles aria-hidden="true" />
            {t(kind.runLabelKey)}
          </Button>
        </>
      )}
      {stage.name === "running" && (
        <Button variant="outline" className={FOOTER_BUTTON} onClick={halt}>
          {t("ai.stop")}
        </Button>
      )}
      {done && (
        <>
          <Button
            variant="outline"
            className={FOOTER_BUTTON}
            disabled={busy}
            onClick={() => setStage({ name: "setup", failure: null })}
          >
            {t("ai.startOver")}
          </Button>
          {kind.primary !== null && done.items.length > 0 ? (
            <PrimaryButton
              icon={kind.primary.icon(env)}
              label={kind.primary.label(selectedItems, env)}
              disabled={busy || selectedItems.length === 0}
              onClick={() => void apply(selectedItems)}
            />
          ) : (
            <PrimaryButton icon={Check} label={t("ai.done")} onClick={onClose} />
          )}
        </>
      )}
    </>
  );

  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose();
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
      {stage.name === "setup" && stage.failure?.kind === "unavailable" && (
        <Callout
          tone="warning"
          size="md"
          announce
          actions={
            <Button
              variant="outline"
              size="sm"
              className="bg-card"
              disabled={problem !== null || stop !== null}
              onClick={() => void start()}
            >
              {t("ai.tryAgain")}
            </Button>
          }
        >
          {t("ai.unavailable")}
        </Callout>
      )}
      {stage.name === "setup" && stage.failure?.kind === "error" && (
        <Callout tone="danger" size="md" announce>
          {stage.failure.message}
        </Callout>
      )}
      {stage.name === "setup" &&
        kind
          .fields(values, set, context, t)
          .map((field, index) => <AiPanelField key={index} field={field} />)}
      {stage.name === "running" && (
        <AiRunSteps steps={kind.stepKeys.map((key) => t(key))} />
      )}
      {done && (
        <>
          {actionError !== null && (
            <Callout tone="danger" size="md" announce>
              {actionError}
            </Callout>
          )}
          {done.notice !== undefined && (
            <Callout tone="info" size="md">
              {done.notice}
            </Callout>
          )}
          <div className="flex min-h-7.5 flex-wrap items-center justify-between gap-2.5">
            <p className="text-muted-fg text-sm">
              {kind.summary(
                done.items.map(({ item }) => item),
                selectedItems.length,
                t,
              )}
            </p>
            {selectable.length > 0 && (
              <button
                type="button"
                className="hover:bg-muted text-meta rounded-seg h-7.5 px-2.5 font-medium whitespace-nowrap"
                onClick={() =>
                  setStage((current) =>
                    current.name === "done"
                      ? { ...current, selected: new Set(allSelected ? [] : selectable) }
                      : current,
                  )
                }
              >
                {allSelected ? t("ai.clearSelection") : t("ai.selectAll")}
              </button>
            )}
          </div>
          {cards.length === 0 && (
            <p className="text-muted-fg text-ui rounded-xl border border-dashed px-4 py-7 text-center">
              {t(kind.emptyKey)}
            </p>
          )}
          {cards.map(({ key, card }) => (
            <AiItemCard
              key={key}
              card={card}
              selected={done.selected.has(key)}
              busy={busy}
              onToggle={() => toggle(key)}
              onAction={(action) => void act(key, action)}
            />
          ))}
        </>
      )}
    </Sheet>
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
    <Button className={FOOTER_BUTTON} disabled={disabled} onClick={onClick}>
      <Icon aria-hidden="true" />
      {label}
    </Button>
  );
}
