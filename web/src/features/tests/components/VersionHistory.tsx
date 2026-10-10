import type { UseQueryResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { CircleDashed, ClipboardList, PencilLine } from "lucide-react";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import type { Test, TestVersion } from "@/features/tests/api";
import { formatDateTime, formatRelative } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { nfc } from "@/lib/nfc";
import { cn } from "@/lib/utils";

/** VersionAction is what a version card's buttons ask the page to confirm. */
export type VersionAction = "draft" | "current" | "delete";

const CARD_BUTTON =
  "h-7 rounded-seg px-2.5 text-xs font-medium in-data-[scale=deck]:px-2.5 in-data-[scale=deck]:text-xs disabled:opacity-45 aria-disabled:cursor-default aria-disabled:opacity-45 aria-disabled:hover:bg-transparent";

/**
 * VersionHistory is the body of the test detail's version history, drawn in
 * the aside and in the sheet alike: the archived note, the draft row while
 * the test has no version or its draft has unpublished changes, and a card
 * per version, newest first. A card names the version, marks the default,
 * gives its questions and points, when and by whom it was published,
 * whether an assignment uses it, and its change note or else the line
 * `summaries` holds for it. Its heading selects it for the preview; Restore
 * as draft, Make default and Delete go to `onAction`. An archived test
 * keeps Restore and Make default off, and a version in use keeps Delete
 * off, each saying why in its title.
 */
export function VersionHistory({
  test,
  query,
  selected,
  summaries,
  pending,
  onSelect,
  onAction,
}: Readonly<{
  test: Test;
  query: UseQueryResult<{ items: TestVersion[] }>;
  selected: number | undefined;
  summaries: ReadonlyMap<number, string>;
  pending: boolean;
  onSelect: (version: number) => void;
  onAction: (kind: VersionAction, version: number) => void;
}>) {
  const { t } = useTranslation();
  const archived = test.status === "archived";
  const unpublished = test.currentVersion === 0;
  const ahead = (test.unpublishedChanges ?? 0) > 0;
  return (
    <div className="flex flex-col gap-1">
      {archived ? (
        <p className="text-muted-fg mb-1.5 text-xs leading-normal">
          {t("tests.detail.history.archived")}
        </p>
      ) : null}
      {unpublished || ahead ? <DraftRow test={test} /> : null}
      {query.isPending ? <ListSkeleton rows={2} /> : null}
      {query.isError ? (
        <LoadError error={query.error} onRetry={() => void query.refetch()}>
          {t("tests.detail.history.failed")}
        </LoadError>
      ) : null}
      {query.isSuccess && query.data.items.length === 0 ? (
        <p className="text-muted-fg text-sm leading-normal">
          {t("tests.detail.history.empty")}
        </p>
      ) : null}
      {query.isSuccess && query.data.items.length > 0 ? (
        <ol className="m-0 flex list-none flex-col gap-1 p-0">
          {query.data.items.map((version) => (
            <VersionCard
              key={version.id}
              version={version}
              isDefault={version.version === test.currentVersion}
              on={version.version === selected}
              summary={summaries.get(version.version)}
              archived={archived}
              pending={pending}
              onSelect={onSelect}
              onAction={onAction}
            />
          ))}
        </ol>
      ) : null}
    </div>
  );
}

function DraftRow({ test }: Readonly<{ test: Test }>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const when = formatRelative(test.updatedAt, locale);
  return (
    <div className="flex items-center gap-2.5 rounded-[9px] border border-dashed px-2.5 py-2.25">
      <PencilLine aria-hidden="true" className="text-muted-fg size-3.75 flex-none" />
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-medium">
          {t("tests.detail.history.draft")}
        </span>
        <span className="text-muted-fg block text-xs">
          {test.currentVersion === 0
            ? t("tests.detail.history.draftNew", { when })
            : t("tests.detail.history.draftAhead", { when })}
        </span>
      </span>
      <Button asChild variant="outline" className={cn(CARD_BUTTON, "shadow-none")}>
        <Link to={`/teacher/tests/${test.id}/edit`}>
          {t("tests.detail.history.edit")}
        </Link>
      </Button>
    </div>
  );
}

function VersionCard({
  version,
  isDefault,
  on,
  summary,
  archived,
  pending,
  onSelect,
  onAction,
}: Readonly<{
  version: TestVersion;
  isDefault: boolean;
  on: boolean;
  summary: string | undefined;
  archived: boolean;
  pending: boolean;
  onSelect: (version: number) => void;
  onAction: (kind: VersionAction, version: number) => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const n = version.version;
  const inUse = version.assignmentCount > 0;
  const note = version.changeNote ? nfc(version.changeNote) : summary;
  const UseIcon = inUse ? ClipboardList : CircleDashed;
  return (
    <li
      data-version={n}
      className={cn(
        "flex flex-col gap-1.5 rounded-lg border p-2",
        on ? "bg-muted border-border" : "border-transparent",
      )}
    >
      <button
        type="button"
        aria-pressed={on}
        onClick={() => onSelect(n)}
        className="flex w-full cursor-pointer items-baseline justify-between gap-2.5 rounded-sm text-left"
      >
        <span className="text-ui flex flex-wrap items-center gap-1.5 font-semibold tabular-nums">
          {t("tests.detail.history.version", { n })}
          {isDefault ? (
            <span className="bg-card text-2xs inline-flex h-5 items-center rounded-[6px] border px-1.75 font-semibold">
              {t("tests.defaultVersion")}
            </span>
          ) : null}
        </span>
        <span className="text-muted-fg text-xs whitespace-nowrap tabular-nums">
          {t("tests.detail.history.stats", {
            questions: version.questionCount,
            points: new Intl.NumberFormat(locale).format(version.totalPoints),
          })}
        </span>
      </button>
      <span className="text-muted-fg text-xs leading-[1.45]">
        {t("tests.detail.history.published", {
          when: formatDateTime(version.publishedAt, locale),
          name: nfc(version.publishedBy),
        })}
      </span>
      <span className="flex flex-wrap items-center gap-1.5">
        <span
          className={cn(
            "text-2xs inline-flex h-5 items-center gap-1.25 rounded-[6px] px-1.75 font-medium whitespace-nowrap",
            inUse ? "bg-info-soft text-info-ink" : "bg-muted text-muted-fg",
          )}
        >
          <UseIcon aria-hidden="true" className="size-2.75 flex-none" />
          {inUse
            ? t("tests.detail.history.inUse", { count: version.assignmentCount })
            : t("tests.detail.history.notUsed")}
        </span>
        {note ? <span className="text-muted-fg text-caption">{note}</span> : null}
      </span>
      <div className="flex flex-wrap gap-1">
        <Button
          variant="outline"
          className={cn(CARD_BUTTON, "shadow-none")}
          aria-disabled={archived || undefined}
          title={archived ? t("tests.detail.history.restoreLocked") : undefined}
          disabled={pending}
          onClick={() => {
            if (!archived) onAction("draft", n);
          }}
        >
          {t("tests.detail.history.restore")}
        </Button>
        {isDefault ? null : (
          <>
            <Button
              variant="ghost"
              className={cn(CARD_BUTTON, "hover:bg-hover")}
              disabled={archived || pending}
              onClick={() => onAction("current", n)}
            >
              {t("tests.detail.history.makeDefault")}
            </Button>
            <Button
              variant="ghost"
              className={cn(
                CARD_BUTTON,
                "text-danger-ink hover:bg-hover hover:text-danger-ink",
              )}
              aria-disabled={inUse || undefined}
              title={inUse ? t("tests.detail.history.deleteLocked") : undefined}
              disabled={pending}
              onClick={() => {
                if (!inUse) onAction("delete", n);
              }}
            >
              {t("tests.detail.history.delete")}
            </Button>
          </>
        )}
      </div>
    </li>
  );
}
