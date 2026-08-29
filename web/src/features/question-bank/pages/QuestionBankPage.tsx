import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Headphones, Play, Plus, Search } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
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
import {
  listQuestions,
  type AdminQuestion,
  type QuestionType,
} from "@/features/question-bank/api";
import { useDebounced } from "@/lib/useDebounced";

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
export default function QuestionBankPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  // Sets, not single values: A-06's rail is checkboxes and chips. Within a
  // group the choices widen the results; the groups narrow each other.
  const [types, setTypes] = useState<readonly QuestionType[]>([]);
  const [tags, setTags] = useState<readonly string[]>([]);
  const [audioOnly, setAudioOnly] = useState(false);
  const [query, setQuery] = useState("");
  const [playing, setPlaying] = useState<string | null>(null);

  // The search fires on a pause, not on a keystroke: §13.8's trigram scan is
  // cheap but not free, and a request per letter is a request per letter.
  const search = useDebounced(query, 300);

  const bank = useInfiniteQuery({
    queryKey: ["admin-questions", { types, tags, audioOnly, search }],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) =>
      listQuestions(
        {
          limit: 50,
          ...(types.length > 0 ? { type: [...types] } : {}),
          ...(tags.length > 0 ? { tag: [...tags] } : {}),
          ...(audioOnly ? { hasAudio: true } : {}),
          ...(search.trim() === "" ? {} : { q: search.trim() }),
          ...(pageParam ? { cursor: pageParam } : {}),
        },
        signal,
      ),
    getNextPageParam: (page) => page.nextCursor ?? undefined,
  });

  const items = bank.data?.pages.flatMap((page) => page.items) ?? [];
  const facets = bank.data?.pages[0]?.facets;
  // Union of what is loaded and what is selected, so a chip cannot vanish from
  // the rail because the rows it filtered to no longer mention it.
  const shownTags = [...new Set([...tags, ...tagsIn(items)])].sort((a, b) =>
    a.localeCompare(b, "vi"),
  );
  const filtering = types.length > 0 || tags.length > 0 || audioOnly;

  return (
    <div className="-m-6 flex h-[calc(100svh-3.5rem)] overflow-hidden">
      {/* Full height with its own scroll: a filter rail that scrolls away
        with the results is a rail you cannot reach while reading them. */}
      <aside className="w-56 shrink-0 space-y-5 overflow-y-auto border-r p-4">
        <div>
          <p className="text-muted-foreground mb-2.5 text-xs font-medium tracking-wide uppercase">
            {t("bank.typeFilter")}
          </p>
          <div className="space-y-2">
            <FilterOption
              label={t("bank.allTypes")}
              count={facets?.all}
              checked={types.length === 0}
              onChange={() => setTypes([])}
            />
            {TYPES.map((value) => (
              <FilterOption
                key={value}
                label={t(`questionEditor.type.${value}`)}
                count={facets?.[value]}
                checked={types.includes(value)}
                onChange={() => setTypes(toggle(types, value))}
              />
            ))}
          </div>
        </div>

        <Separator />

        <div>
          <p className="text-muted-foreground mb-2.5 text-xs font-medium tracking-wide uppercase">
            {t("bank.tagFilter")}
          </p>
          {!bank.isSuccess ? null : shownTags.length === 0 ? (
            <p className="text-muted-foreground text-xs">{t("bank.noTags")}</p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {shownTags.map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={tags.includes(value)}
                  onClick={() => setTags(toggle(tags, value))}
                >
                  <Badge variant={tags.includes(value) ? "primary" : "outline"}>
                    {value}
                  </Badge>
                </button>
              ))}
            </div>
          )}
        </div>

        <Separator />

        <label className="flex items-center gap-2.5 text-sm">
          <Checkbox
            checked={audioOnly}
            onChange={(event) => setAudioOnly(event.target.checked)}
          />
          {t("bank.audioOnly")}
        </label>
      </aside>

      <div className="min-w-0 flex-1 space-y-4 overflow-y-auto p-6">
        <div className="flex items-end justify-between">
          <div>
            <h1 className="text-xl font-semibold tracking-tight">
              {t("nav.questionBank")}
            </h1>
            <p className="text-muted-foreground mt-0.5 text-sm">
              {facets ? t("bank.summary", { count: facets.all }) : "\u00a0"}
            </p>
          </div>
          <Button asChild size="sm">
            <Link to="/admin/question-bank/new">
              <Plus aria-hidden="true" />
              {t("bank.newQuestion")}
            </Link>
          </Button>
        </div>

        <div className="relative">
          <Search
            className="text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4"
            aria-hidden="true"
          />
          <Input
            className="pl-9"
            value={query}
            placeholder={t("bank.searchPlaceholder")}
            aria-label={t("bank.searchPlaceholder")}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>

        {bank.isPending ? (
          <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
            {t("common.loading")}
          </p>
        ) : bank.isError ? (
          <div className="space-y-3">
            <p role="alert" className="text-sm">
              {t("bank.loadFailed")}
            </p>
            <Button variant="outline" size="sm" onClick={() => void bank.refetch()}>
              {t("common.retry")}
            </Button>
          </div>
        ) : items.length === 0 ? (
          // §12: one short sentence and one action, no illustration.
          <div className="space-y-3">
            <p className="text-muted-foreground text-sm">
              {filtering || search.trim() !== ""
                ? t("bank.noMatches")
                : t("bank.empty")}
            </p>
            <Button asChild size="sm">
              <Link to="/admin/question-bank/new">{t("bank.newQuestion")}</Link>
            </Button>
          </div>
        ) : (
          <>
            <Card className="gap-0 overflow-hidden py-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[42%]">{t("bank.prompt")}</TableHead>
                    <TableHead>{t("bank.type")}</TableHead>
                    <TableHead>{t("bank.tags")}</TableHead>
                    <TableHead className="text-right">{t("bank.points")}</TableHead>
                    <TableHead>{t("bank.usedIn")}</TableHead>
                    <TableHead className="w-9">
                      <span className="sr-only">{t("bank.preview")}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((question) => (
                    <Row
                      key={question.id}
                      question={question}
                      playing={playing === question.id}
                      onOpen={() =>
                        void navigate(`/admin/question-bank/${question.id}`)
                      }
                      onRetry={() => void bank.refetch()}
                      onTogglePlay={() =>
                        setPlaying(playing === question.id ? null : question.id)
                      }
                    />
                  ))}
                </TableBody>
              </Table>
            </Card>

            {bank.hasNextPage ? (
              <Button
                variant="outline"
                size="sm"
                disabled={bank.isFetchingNextPage}
                onClick={() => void bank.fetchNextPage()}
              >
                {bank.isFetchingNextPage ? t("common.loading") : t("bank.loadMore")}
              </Button>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}

function Row({
  question,
  playing,
  onOpen,
  onRetry,
  onTogglePlay,
}: {
  question: AdminQuestion;
  playing: boolean;
  onOpen: () => void;
  onRetry: () => void;
  onTogglePlay: () => void;
}) {
  const { t } = useTranslation();
  const audio = question.media?.kind === "audio" ? question.media : null;

  return (
    <>
      <TableRow>
        <TableCell>
          <div className="flex items-center gap-2">
            {audio ? (
              <Headphones
                className="text-muted-foreground size-3.5 shrink-0"
                aria-hidden="true"
              />
            ) : null}
            <button type="button" className="truncate text-left" onClick={onOpen}>
              {question.prompt}
            </button>
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
          {question.usedInTests === undefined
            ? "—"
            : question.usedInTests === 0
              ? "—"
              : t("bank.usedInCount", { count: question.usedInTests })}
        </TableCell>
        <TableCell className="text-right">
          {audio ? (
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={t("bank.previewOf", { prompt: question.prompt })}
              aria-pressed={playing}
              onClick={onTogglePlay}
            >
              <Play aria-hidden="true" />
            </Button>
          ) : null}
        </TableCell>
      </TableRow>

      {audio && playing ? (
        <TableRow>
          <TableCell colSpan={6} className="p-0">
            {/* Keyed on the URL: a refetch mints a fresh one, and the row has
                to forget that the previous one had expired. */}
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
function tagsIn(questions: AdminQuestion[]): string[] {
  const seen = new Set<string>();
  for (const question of questions) for (const tag of question.tags) seen.add(tag);
  return [...seen].sort((a, b) => a.localeCompare(b, "vi"));
}

function FilterOption({
  label,
  count,
  checked,
  onChange,
}: {
  label: string;
  count: number | undefined;
  checked: boolean;
  onChange: () => void;
}) {
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
