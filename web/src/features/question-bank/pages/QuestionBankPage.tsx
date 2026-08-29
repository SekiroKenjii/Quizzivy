import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Headphones, Play, Plus, Search } from "lucide-react";
import { Badge } from "@/components/ui/badge";
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

  const [type, setType] = useState<QuestionType | null>(null);
  const [tag, setTag] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [playing, setPlaying] = useState<string | null>(null);

  // The search fires on a pause, not on a keystroke: §13.8's trigram scan is
  // cheap but not free, and a request per letter is a request per letter.
  const search = useDebounced(query, 300);

  const bank = useInfiniteQuery({
    queryKey: ["admin-questions", { type, tag, search }],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) =>
      listQuestions(
        {
          limit: 50,
          ...(type ? { type } : {}),
          ...(tag ? { tag } : {}),
          ...(search.trim() === "" ? {} : { q: search.trim() }),
          ...(pageParam ? { cursor: pageParam } : {}),
        },
        signal,
      ),
    getNextPageParam: (page) => page.nextCursor ?? undefined,
  });

  const items = bank.data?.pages.flatMap((page) => page.items) ?? [];
  const tags = tagsIn(items);

  return (
    <div className="-m-6 flex">
      <aside className="w-56 shrink-0 space-y-5 border-r p-4">
        <div>
          <p className="text-muted-foreground mb-2.5 text-xs font-medium tracking-wide uppercase">
            {t("bank.typeFilter")}
          </p>
          <div className="space-y-2">
            <FilterOption
              label={t("bank.allTypes")}
              checked={type === null}
              onChange={() => setType(null)}
            />
            {TYPES.map((value) => (
              <FilterOption
                key={value}
                label={t(`questionEditor.type.${value}`)}
                checked={type === value}
                onChange={() => setType(type === value ? null : value)}
              />
            ))}
          </div>
        </div>

        <Separator />

        <div>
          <p className="text-muted-foreground mb-2.5 text-xs font-medium tracking-wide uppercase">
            {t("bank.tagFilter")}
          </p>
          {!bank.isSuccess ? null : tags.length === 0 ? (
            <p className="text-muted-foreground text-xs">{t("bank.noTags")}</p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {tags.map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={tag === value}
                  onClick={() => setTag(tag === value ? null : value)}
                >
                  <Badge variant={tag === value ? "primary" : "outline"}>{value}</Badge>
                </button>
              ))}
            </div>
          )}
        </div>
      </aside>

      <div className="min-w-0 flex-1 space-y-4 p-6">
        <div className="flex items-end justify-between">
          <div>
            <h1 className="text-xl font-semibold tracking-tight">
              {t("nav.questionBank")}
            </h1>
            <p className="text-muted-foreground mt-0.5 text-sm">
              {bank.isSuccess
                ? t(bank.hasNextPage ? "bank.summarySoFar" : "bank.summary", {
                    count: items.length,
                  })
                : "\u00a0"}
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
              {type !== null || tag !== null || search.trim() !== ""
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
          <TableCell colSpan={5} className="p-0">
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
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: () => void;
}) {
  return (
    <label className="flex items-center gap-2.5 text-sm">
      <input
        type="checkbox"
        checked={checked}
        onChange={onChange}
        className="border-input accent-foreground size-4"
      />
      {label}
    </label>
  );
}
