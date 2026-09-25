import { BankNavigation } from "@/features/question-groups/components/BankNavigation";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { BulkActions } from "@/components/shared/BulkActions";
import { formatRelative } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { useListFilters } from "@/hooks/useListFilters";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  ArrowUpRight,
  ChevronDown,
  Copy,
  Play,
  Plus,
  Tag as TagIcon,
  Trash2,
  X,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { AddToTestDialog } from "@/features/question-bank/components/AddToTestDialog";
import { BulkTagDialog } from "@/features/question-bank/components/BulkTagDialog";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { AudioPreviewRow } from "@/features/question-bank/components/AudioPreviewRow";
import { QuestionUsageRow } from "@/features/question-bank/components/QuestionUsageRow";
import {
  deleteQuestion,
  duplicateQuestion,
  listQuestions,
  type AdminQuestion,
  type QuestionType,
} from "@/features/question-bank/api";
import { useDebounced } from "@/lib/useDebounced";
import { PageAside } from "@/components/shared/PageAside";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { RowMenu } from "@/components/shared/RowMenu";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/sonner";
import { ApiError, referencingTests, type ReferencingTest } from "@/lib/api/errors";
import { PageHeader } from "@/components/shared/PageHeader";
import { SearchInput } from "@/components/shared/SearchInput";
import { Pager } from "@/components/shared/Pager";
import { usePage } from "@/hooks/usePage";
import type { TFunction } from "i18next";

const TYPES: QuestionType[] = [
  "single_choice",
  "multiple_choice",
  "true_false",
  "fill_blank",
  "short_answer",
];

/**
 * §8's question bank, as the deck's A-06.
 *
 * Filters on the left, results on the right: the second test is faster than the
 * first only if last term's work is findable.
 */
const PAGE_SIZE = 20;

export default function QuestionBankPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  // Sets, not single values: A-06's rail is checkboxes and chips.
  const { params, setFilter } = useListFilters();
  const types = params
    .getAll("type")
    .filter((value): value is QuestionType => TYPES.some((type) => type === value));
  const setTypes = (value: readonly QuestionType[]) => setFilter("type", value);
  const tags = params.getAll("tag");
  const setTags = (value: readonly string[]) => setFilter("tag", value);
  const audioOnly = params.get("hasAudio") === "true";
  const setAudioOnly = (value: boolean) => setFilter("hasAudio", value ? "true" : null);
  const bulk = useBulkSelection<AdminQuestion>();
  const selected = new Set(bulk.selected.keys());
  const [tagging, setTagging] = useState(false);
  const [addingOne, setAddingOne] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<AdminQuestion | null>(null);
  const [blocked, setBlocked] = useState<ReferencingTest[] | null>(null);
  const queryClient = useQueryClient();
  const remove = useMutation({
    mutationFn: (id: string) => deleteQuestion(id),
    onSuccess: async () => {
      setDeleting(null);
      await queryClient.invalidateQueries({ queryKey: ["admin-questions"] });
      toast(t("bank.deleted"));
    },
    onError: (cause) => {
      if (cause instanceof ApiError && cause.code === "QUESTION_REFERENCED")
        setBlocked(referencingTests(cause));
      else toast(t("bank.deleteFailed"));
    },
  });
  const [duplicated, setDuplicated] = useState<ReadonlySet<string>>(new Set());
  const duplicate = useMutation({
    mutationFn: (id: string) => duplicateQuestion(id),
    onSuccess: async (copy, sourceId) => {
      setDuplicated((current) => new Set([...current, sourceId, copy.id]));
      await queryClient.invalidateQueries({ queryKey: ["admin-questions"] });
      toast(t("bank.duplicated"));
    },
    onError: () => toast(t("bank.duplicateFailed")),
  });
  const [adding, setAdding] = useState(false);
  const query = params.get("q") ?? "";
  const setQuery = (value: string) => setFilter("q", value);
  const [playing, setPlaying] = useState<string | null>(null);

  const search = useDebounced(query, 300);

  const [page] = usePage(
    JSON.stringify({
      types: [...types],
      tags: [...tags],
      audioOnly,
      search: search.trim(),
    }),
  );
  const bank = useQuery({
    queryKey: ["admin-questions", { types, tags, audioOnly, search, page }],
    queryFn: ({ signal }) =>
      listQuestions(
        {
          limit: PAGE_SIZE,
          page,
          ...(types.length > 0 ? { type: [...types] } : {}),
          ...(tags.length > 0 ? { tag: [...tags] } : {}),
          ...(audioOnly ? { hasAudio: true } : {}),
          ...(search.trim() === "" ? {} : { q: search.trim() }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
  });

  const items = bank.data?.items ?? [];
  const data = bank.data;
  const selectPage = (checked: boolean) => bulk.selectPage(items, checked);
  const toggleSelected = (id: string) => {
    const item = items.find((q) => q.id === id);
    if (item) bulk.toggle(item);
  };
  const facets = data?.facets;
  // From the server, not from `items`.
  const shownTags = [...new Set([...tags, ...(bank.data?.tags ?? [])])].sort((a, b) =>
    a.localeCompare(b, "vi"),
  );
  const filtering = types.length > 0 || tags.length > 0 || audioOnly;
  const allSelected = items.length > 0 && items.every((q) => selected.has(q.id));

  return (
    <>
      <FilterRail
        facets={facets}
        types={types}
        tags={tags}
        shownTags={shownTags}
        tagsReady={bank.isSuccess}
        audioOnly={audioOnly}
        onTypes={setTypes}
        onTags={setTags}
        onAudioOnly={setAudioOnly}
      />

      <div className="space-y-4">
        <PageHeader
          variant="title"
          title={t("nav.questionBank")}
          subtitle={bankSubtitle(data, t)}
          actions={
            <Button asChild size="sm">
              <Link to="/admin/question-bank/new">
                <Plus aria-hidden="true" />
                {t("bank.newQuestion")}
              </Link>
            </Button>
          }
        />

        <BankNavigation />
        <SearchInput
          className="w-full"
          value={query}
          onChange={setQuery}
          placeholder={t("bank.searchPlaceholder")}
        />

        <BulkActions
          selected={[...bulk.selected.values()]}
          name={(item) => item.prompt}
          selectionLabel={t("bank.selectedCount", { count: selected.size })}
          actions={[
            {
              label: t("common.bulkDelete"),
              description: t("common.permanentDeleteBody"),
              run: (item) => deleteQuestion(item.id),
            },
          ]}
          onRemoved={bulk.remove}
          onClear={bulk.clear}
          onSettled={() =>
            queryClient.invalidateQueries({ queryKey: ["admin-questions"] })
          }
        >
          <Button variant="outline" size="sm" onClick={() => setAdding(true)}>
            <Plus aria-hidden="true" />
            {t("bank.addToTest")}
          </Button>
          <Button variant="outline" size="sm" onClick={() => setTagging(true)}>
            <TagIcon aria-hidden="true" />
            {t("bank.bulkTag")}
          </Button>
        </BulkActions>
        <QueryStates
          query={bank}
          skeleton={<ListSkeleton />}
          failed={t("bank.loadFailed")}
        >
          {(data) =>
            items.length === 0 ? (
              <EmptyState
                action={
                  <Button asChild size="sm">
                    <Link to="/admin/question-bank/new">{t("bank.newQuestion")}</Link>
                  </Button>
                }
              >
                {filtering || search.trim() !== ""
                  ? t("bank.noMatches")
                  : t("bank.empty")}
              </EmptyState>
            ) : (
              <>
                <Card className="gap-0 overflow-hidden py-0">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-9">
                          <Checkbox
                            aria-label={t("bank.selectAll")}
                            checked={allSelected}
                            onChange={(event) => selectPage(event.target.checked)}
                          />
                        </TableHead>
                        <TableHead className="w-[42%]">{t("bank.prompt")}</TableHead>
                        <TableHead>{t("bank.type")}</TableHead>
                        <TableHead>{t("bank.tags")}</TableHead>
                        <TableHead className="text-right">{t("bank.points")}</TableHead>
                        <TableHead>{t("bank.usedIn")}</TableHead>
                        <TableHead>{t("common.updated")}</TableHead>
                        <TableHead className="w-10" />
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {items.map((question) => (
                        <Row
                          key={question.id}
                          selected={selected.has(question.id)}
                          onToggleSelect={() => toggleSelected(question.id)}
                          question={question}
                          duplicated={duplicated.has(question.id)}
                          duplicating={duplicate.isPending}
                          playing={playing === question.id}
                          onOpen={() =>
                            void navigate(`/admin/question-bank/${question.id}`)
                          }
                          onRetry={() => void bank.refetch()}
                          onTogglePlay={() =>
                            setPlaying(playing === question.id ? null : question.id)
                          }
                          onAddToTest={() => setAddingOne(question.id)}
                          onDuplicate={() => duplicate.mutate(question.id)}
                          onDelete={() => {
                            setBlocked(null);
                            setDeleting(question);
                          }}
                        />
                      ))}
                    </TableBody>
                  </Table>
                </Card>

                {data && (
                  <Pager page={data.page} pageSize={data.pageSize} total={data.total} />
                )}
              </>
            )
          }
        </QueryStates>
      </div>
      <BulkTagDialog
        questionIds={[...selected]}
        suggestions={shownTags}
        open={tagging}
        onOpenChange={setTagging}
        onApplied={() => bulk.clear()}
      />
      <AddToTestDialog
        questionIds={addingOne === null ? [...selected] : [addingOne]}
        open={adding || addingOne !== null}
        onOpenChange={(open) => {
          if (open) return;
          setAdding(false);
          setAddingOne(null);
        }}
        onAdded={() => {
          if (addingOne === null) bulk.clear();
        }}
      />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => !open && setDeleting(null)}
        title={t(blocked ? "bank.deleteBlockedTitle" : "bank.deleteConfirmTitle")}
        description={t(blocked ? "bank.deleteBlockedBody" : "bank.deleteConfirmBody")}
        confirmLabel={t(blocked ? "common.close" : "bank.delete")}
        destructive={blocked === null}
        pending={remove.isPending}
        {...(blocked === null
          ? { onConfirm: () => deleting && remove.mutate(deleting.id) }
          : {})}
      >
        <p className="bg-muted truncate rounded-md px-3 py-2 text-sm">
          {deleting?.prompt}
        </p>
        {/* A-06a: the drafts that block it, as links, instead of a button that cannot fire. */}
        {blocked === null || blocked.length === 0 ? null : (
          <p className="text-sm">
            {t("bank.deleteBlockedList", { count: blocked.length })}{" "}
            {blocked.map((test, index) => (
              <span key={test.id}>
                {index === 0 ? null : ", "}
                <Link
                  to={`/admin/tests/${test.id}/edit`}
                  className="font-medium underline underline-offset-4"
                >
                  {test.title}
                </Link>
              </span>
            ))}
          </p>
        )}
      </ConfirmDialog>
    </>
  );
}

function Row({
  question,
  duplicated,
  duplicating,
  playing,
  onOpen,
  onRetry,
  onTogglePlay,
  selected,
  onToggleSelect,
  onAddToTest,
  onDuplicate,
  onDelete,
}: Readonly<{
  question: AdminQuestion;
  duplicated: boolean;
  duplicating: boolean;
  playing: boolean;
  selected: boolean;
  onOpen: () => void;
  onRetry: () => void;
  onTogglePlay: () => void;
  onToggleSelect: () => void;
  onAddToTest: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const audio = question.media?.kind === "audio" ? question.media : null;
  const [usageOpen, setUsageOpen] = useState(false);

  return (
    <>
      <TableRow>
        <TableCell>
          <Checkbox
            checked={selected}
            aria-label={t("bank.selectQuestion", { prompt: question.prompt })}
            onChange={onToggleSelect}
          />
        </TableCell>
        <TableCell>
          <div className="flex items-center gap-2">
            {audio ? (
              <Button
                variant="ghost"
                size="icon-xs"
                className="text-muted-foreground shrink-0"
                aria-label={t("bank.previewOf", { prompt: question.prompt })}
                aria-pressed={playing}
                onClick={onTogglePlay}
              >
                <Play aria-hidden="true" />
              </Button>
            ) : null}
            <Link
              to={`/admin/question-bank/${question.id}`}
              className="truncate hover:underline"
            >
              {question.prompt.replace(/\{\{\d+\}\}/g, "___")}
            </Link>
            {duplicated ? (
              <Badge variant="outline">{t("common.justDuplicated")}</Badge>
            ) : null}
          </div>
        </TableCell>
        <TableCell className="text-muted-foreground">
          {t(`questionEditor.type.${question.type}`)}
        </TableCell>
        <TableCell>
          <span className="flex flex-wrap gap-1">
            {question.tags.map((value) => (
              <Badge key={value}>{value}</Badge>
            ))}
          </span>
        </TableCell>
        <TableCell className="text-right tabular-nums">{question.points}</TableCell>
        <TableCell className="text-muted-foreground tabular-nums">
          {question.usedInTests ? (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 gap-1 px-1.5"
              aria-expanded={usageOpen}
              aria-controls={`question-usage-${question.id}`}
              onClick={() => setUsageOpen(!usageOpen)}
            >
              <ChevronDown
                className={usageOpen ? "rotate-180" : ""}
                aria-hidden="true"
              />
              {t("bank.usedInCount", { count: question.usedInTests })}
            </Button>
          ) : (
            "—"
          )}
        </TableCell>
        <TableCell className="text-muted-foreground">
          {formatRelative(question.updatedAt, locale)}
        </TableCell>
        <TableCell className="text-right">
          <div className="flex items-center justify-end gap-1">
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={t("common.duplicateNamed", { name: question.prompt })}
              disabled={duplicating}
              onClick={onDuplicate}
            >
              <Copy aria-hidden="true" />
            </Button>
            <RowMenu>
              <DropdownMenuItem onSelect={onOpen}>
                <ArrowUpRight className="text-muted-foreground" aria-hidden="true" />
                {t("bank.open")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={onAddToTest}>
                <Plus className="text-muted-foreground" aria-hidden="true" />
                {t("bank.addToTest")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={onDuplicate}>
                <Copy className="text-muted-foreground" aria-hidden="true" />
                {t("bank.duplicate")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={onDelete}>
                <Trash2 aria-hidden="true" />
                {t("bank.delete")}
              </DropdownMenuItem>
            </RowMenu>
          </div>
        </TableCell>
      </TableRow>

      {usageOpen ? (
        <QuestionUsageRow questionId={question.id} prompt={question.prompt} />
      ) : null}
      {audio && playing ? (
        <TableRow>
          <TableCell colSpan={8} className="p-0">
            <AudioPreviewRow
              key={audio.url}
              asset={audio}
              onRetry={onRetry}
              onClose={onTogglePlay}
            />
          </TableCell>
        </TableRow>
      ) : null}
    </>
  );
}

/** The tags actually present in the results, so the rail cannot offer a dead end. */

function FilterRail({
  facets,
  types,
  tags,
  shownTags,
  tagsReady,
  audioOnly,
  onTypes,
  onTags,
  onAudioOnly,
}: Readonly<{
  facets: Record<string, number> | undefined;
  types: readonly QuestionType[];
  tags: readonly string[];
  shownTags: readonly string[];
  tagsReady: boolean;
  audioOnly: boolean;
  onTypes: (next: readonly QuestionType[]) => void;
  onTags: (next: readonly string[]) => void;
  onAudioOnly: (next: boolean) => void;
}>) {
  const { t } = useTranslation();
  return (
    <PageAside side="left" label={t("bank.filters")}>
      <div>
        <p className="text-muted-foreground mb-3 text-xs font-medium tracking-wide uppercase">
          {t("bank.typeFilter")}
        </p>
        <div className="space-y-3">
          {TYPES.map((value) => (
            <FilterOption
              key={value}
              label={t(`questionEditor.type.${value}`)}
              count={facets?.[value]}
              checked={types.includes(value)}
              onChange={() => onTypes(toggle(types, value))}
            />
          ))}
        </div>
      </div>

      <Separator />

      <div>
        <p className="text-muted-foreground mb-2.5 text-xs font-medium tracking-wide uppercase">
          {t("bank.tagFilter")}
        </p>
        {tagsReady &&
          (shownTags.length === 0 ? (
            <p className="text-muted-foreground text-xs">{t("bank.noTags")}</p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {shownTags.map((value) => {
                const picked = tags.includes(value);
                return (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={picked}
                    onClick={() => onTags(toggle(tags, value))}
                  >
                    <Badge variant={picked ? "primary" : "outline"} className="gap-1">
                      {value}
                      {picked ? <X className="size-3" aria-hidden="true" /> : null}
                    </Badge>
                  </button>
                );
              })}
            </div>
          ))}
      </div>

      <Separator />

      <label className="flex items-center gap-2.5 text-sm">
        <Checkbox
          checked={audioOnly}
          onChange={(event) => onAudioOnly(event.target.checked)}
        />
        {t("bank.audioOnly")}
      </label>
    </PageAside>
  );
}

function FilterOption({
  label,
  count,
  checked,
  onChange,
}: Readonly<{
  label: string;
  count: number | undefined;
  checked: boolean;
  onChange: () => void;
}>) {
  return (
    <label className="flex items-center gap-2.5 text-sm">
      <Checkbox checked={checked} onChange={onChange} />
      {label}
      {count === undefined ? null : (
        <span className="text-muted-foreground ml-auto text-xs tabular-nums">
          {count}
        </span>
      )}
    </label>
  );
}

/** Adds or removes one value, keeping the rest. */
function toggle<T>(values: readonly T[], value: T): readonly T[] {
  return values.includes(value)
    ? values.filter((v) => v !== value)
    : [...values, value];
}

function bankSubtitle(
  data: { readonly total: number; readonly bankTotal: number } | undefined,
  t: TFunction,
): string {
  if (data === undefined) return "\u00a0";
  if (data.total === data.bankTotal)
    return t("bank.summary", { count: data.bankTotal });
  return t("bank.summaryFiltered", { count: data.bankTotal, filtered: data.total });
}
