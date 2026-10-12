import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useInfiniteQuery } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { Check, Library } from "lucide-react";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { SearchInput } from "@/components/shared/SearchInput";
import { Button } from "@/components/ui/button";
import { listQuestions, type AdminQuestion } from "@/features/question-bank/api";
import { useDebounced } from "@/lib/useDebounced";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 50;

interface QuestionPickerDialogProps {
  open: boolean;
  /** Already in the outline: shown, marked, and not offered again. */
  excluded: ReadonlySet<string>;
  /** The section the questions join, named in the description. */
  destination: string | null;
  onOpenChange: (open: boolean) => void;
  onPick: (questionIds: string[]) => void;
  onPickGroup?: () => void;
  /** Takes focus back when the dialog closes, if it is still in the page. */
  returnFocus?: RefObject<HTMLElement | null>;
}

/**
 * QuestionPickerDialog is "Add from question bank": the caller's whole bank,
 * searched on the server (debounced 250ms) and paged as the list scrolls,
 * each row with "{type} · {level} · used in {n} tests". Any number can be
 * ticked; ticks survive a new search and later pages, and "Add {n}
 * questions" sends every ticked id in the order ticked. A question the test
 * already holds is marked "In this test" and cannot be ticked.
 */
export function QuestionPickerDialog({
  open,
  excluded,
  destination,
  onOpenChange,
  onPick,
  onPickGroup,
  returnFocus,
}: Readonly<QuestionPickerDialogProps>) {
  const { t } = useTranslation();
  return (
    <DialogShell
      open={open}
      onOpenChange={onOpenChange}
      width={560}
      returnFocus={returnFocus}
    >
      <DialogShellHeader
        icon={Library}
        title={t("builder.bank.title")}
        description={
          destination === null
            ? t("builder.bank.descriptionAny")
            : t("builder.bank.description", { section: destination })
        }
      />
      {open ? (
        <Picker
          excluded={excluded}
          onCancel={() => onOpenChange(false)}
          onPickGroup={onPickGroup}
          onAdd={(ids) => {
            onPick(ids);
            onOpenChange(false);
          }}
        />
      ) : null}
    </DialogShell>
  );
}

function Picker({
  excluded,
  onCancel,
  onPickGroup,
  onAdd,
}: Readonly<{
  excluded: ReadonlySet<string>;
  onCancel: () => void;
  onPickGroup: (() => void) | undefined;
  onAdd: (ids: string[]) => void;
}>) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [ticked, setTicked] = useState<readonly string[]>([]);
  const search = useDebounced(query.trim(), 250);
  const bank = useInfiniteQuery({
    queryKey: ["admin-questions", "picker", search],
    initialPageParam: 1,
    queryFn: ({ pageParam, signal }) =>
      listQuestions(
        { ...(search === "" ? {} : { q: search }), page: pageParam, limit: PAGE_SIZE },
        signal,
      ),
    getNextPageParam: (last) =>
      last.page * last.pageSize < last.total ? last.page + 1 : undefined,
  });
  const items = bank.data?.pages.flatMap((page) => page.items) ?? [];
  const chosen = new Set(ticked);

  function toggle(id: string) {
    setTicked((current) =>
      current.includes(id) ? current.filter((item) => item !== id) : [...current, id],
    );
  }

  return (
    <>
      <DialogShellBody>
        <div className="flex flex-wrap items-center gap-2.5">
          <SearchInput
            value={query}
            onChange={setQuery}
            placeholder={t("builder.bank.search")}
            dense
            className="min-w-0 flex-1"
          />
          {ticked.length > 0 ? (
            <span role="status" className="text-muted-fg text-xs whitespace-nowrap">
              {t("formDialog.selected", { count: ticked.length })}
            </span>
          ) : null}
        </div>
        <BankRows
          status={bank.status}
          items={items}
          searching={search !== ""}
          excluded={excluded}
          chosen={chosen}
          hasMore={bank.hasNextPage}
          loadingMore={bank.isFetchingNextPage}
          onRetry={() => void bank.refetch()}
          onLoadMore={() => {
            if (bank.hasNextPage) void bank.fetchNextPage({ cancelRefetch: false });
          }}
          onToggle={toggle}
        />
        {onPickGroup ? (
          <Button
            type="button"
            variant="outline"
            className="self-start"
            onClick={onPickGroup}
          >
            {t("builder.chooseWholeGroup")}
          </Button>
        ) : null}
      </DialogShellBody>
      <DialogShellFooter>
        <Button type="button" variant="outline" onClick={onCancel}>
          {t("common.cancel")}
        </Button>
        <Button
          type="button"
          disabled={ticked.length === 0}
          onClick={() => onAdd([...ticked])}
        >
          {ticked.length > 0
            ? t("builder.bank.add", { count: ticked.length })
            : t("builder.bank.addNone")}
        </Button>
      </DialogShellFooter>
    </>
  );
}

function BankRows({
  status,
  items,
  searching,
  excluded,
  chosen,
  hasMore,
  loadingMore,
  onRetry,
  onLoadMore,
  onToggle,
}: Readonly<{
  status: "pending" | "error" | "success";
  items: AdminQuestion[];
  searching: boolean;
  excluded: ReadonlySet<string>;
  chosen: ReadonlySet<string>;
  hasMore: boolean;
  loadingMore: boolean;
  onRetry: () => void;
  onLoadMore: () => void;
  onToggle: (id: string) => void;
}>) {
  const { t } = useTranslation();
  if (status === "pending")
    return (
      <p className="text-muted-fg text-sm" role="status" aria-live="polite">
        {t("common.loading")}
      </p>
    );
  if (status === "error")
    return (
      <div role="alert" className="flex items-center gap-3 text-sm">
        <span className="text-danger-ink flex-1">{t("builder.bankFailed")}</span>
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          {t("common.retry")}
        </Button>
      </div>
    );
  if (items.length === 0)
    return (
      <p className="text-muted-fg text-sm">
        {t(searching ? "builder.bankNoMatches" : "builder.bankEmpty")}
      </p>
    );
  return (
    <ul
      aria-label={t("builder.bank.list")}
      className="flex max-h-75 flex-col overflow-y-auto rounded-lg border"
    >
      {items.map((question) => {
        const inTest = excluded.has(question.id);
        const on = chosen.has(question.id);
        return (
          <li key={question.id} className="border-t first:border-t-0">
            <button
              type="button"
              role="checkbox"
              aria-checked={on}
              disabled={inTest}
              onClick={() => onToggle(question.id)}
              className="hover:bg-muted flex w-full items-center gap-2.5 px-3 py-2.5 text-left disabled:cursor-default disabled:opacity-60 disabled:hover:bg-transparent"
            >
              <span
                aria-hidden="true"
                className={cn(
                  "grid size-4 flex-none place-items-center rounded-[4px] border",
                  on
                    ? "bg-primary border-primary text-primary-foreground"
                    : "border-ring bg-card",
                )}
              >
                {on ? <Check className="size-3" /> : null}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm">{question.prompt}</span>
                <span className="text-muted-fg block truncate text-xs">
                  {inTest ? t("builder.bank.inTest") : questionMeta(question, t)}
                </span>
              </span>
            </button>
          </li>
        );
      })}
      <LoadMoreSentinel
        as="li"
        active={hasMore}
        loading={loadingMore}
        onVisible={onLoadMore}
      />
    </ul>
  );
}

function questionMeta(question: AdminQuestion, t: TFunction): string {
  return [
    t(`questionEditor.type.${question.type}`),
    question.level ? t(`bank.level.${question.level}`) : null,
    t("builder.bank.usedIn", { count: question.usedInTests ?? 0 }),
  ]
    .filter(Boolean)
    .join(" · ");
}
