import { useEffect, useId, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Search } from "lucide-react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Kbd } from "@/components/ui/kbd";
import { can, hasWorkspace } from "@/features/auth/permissions";
import { listQuestions } from "@/features/question-bank/api";
import { listStudents } from "@/features/students/api";
import { listTests } from "@/features/tests/api";
import { useDebounced } from "@/lib/useDebounced";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth";
import {
  PALETTE_GROUPS,
  PALETTE_LIMITS,
  assignmentEntries,
  pageEntries,
  palettePages,
  questionEntries,
  searchAssignments,
  studentEntries,
  testEntries,
  type PaletteEntry,
  type PaletteGroup,
} from "./paletteResults";

const DEBOUNCE_MS = 250;
const STALE_MS = 30_000;

function usePaletteEntries(
  open: boolean,
  search: string,
): {
  entries: PaletteEntry[];
  settled: boolean;
} {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const teacher = hasWorkspace(user, "teacher");
  const students = can(user, "people.students.read");
  const pages = useMemo(() => palettePages(user, t), [user, t]);
  const searching = search !== "";
  const common = { placeholderData: keepPreviousData, staleTime: STALE_MS };

  const assignments = useQuery({
    queryKey: ["palette", "assignments", search],
    queryFn: ({ signal }) => searchAssignments(search, signal),
    enabled: open && teacher,
    ...common,
  });
  const people = useQuery({
    queryKey: ["palette", "students", search],
    queryFn: ({ signal }) =>
      listStudents(
        { limit: PALETTE_LIMITS.students, ...(searching ? { q: search } : {}) },
        signal,
      ),
    enabled: open && students,
    ...common,
  });
  const tests = useQuery({
    queryKey: ["palette", "tests", search],
    queryFn: ({ signal }) =>
      listTests({ q: search, limit: PALETTE_LIMITS.tests }, signal),
    enabled: open && teacher && searching,
    ...common,
  });
  const questions = useQuery({
    queryKey: ["palette", "questions", search],
    queryFn: ({ signal }) =>
      listQuestions({ q: search, limit: PALETTE_LIMITS.questions }, signal),
    enabled: open && teacher && searching,
    ...common,
  });

  const now = new Date();
  const entries = [
    ...pageEntries(pages, search),
    ...(teacher ? assignmentEntries(assignments.data ?? [], now, t) : []),
    ...(students ? studentEntries(people.data?.items ?? []) : []),
    ...(teacher && searching ? testEntries(tests.data?.items ?? [], t) : []),
    ...(teacher && searching ? questionEntries(questions.data?.items ?? []) : []),
  ];
  const settled = ![assignments, people, tests, questions].some(
    (query) => query.isFetching,
  );
  return { entries, settled };
}

/**
 * CommandPalette is the teacher's palette as the deck draws it: a 560px panel
 * 12% from the top with "Search pages, tests, students…". It lists the pages
 * the user may open, then up to 4 assignments with their status and up to 3
 * students with their first class, and, once something is typed, up to 4
 * tests and 3 questions. Pages match on the client and the rest on the
 * server, both ignoring accents and case; each search waits for the typing to
 * pause, and an answer to an older search never replaces a newer one. ↑ and ↓
 * move the highlight, Enter opens it, Esc closes, and "Nothing matches" says
 * when a search finds nothing (DG-36).
 */
export function CommandPalette({
  open,
  onOpenChange,
}: Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
}>) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);
  const listId = useId();
  const search = useDebounced(query.trim(), DEBOUNCE_MS);
  const { entries, settled } = usePaletteEntries(open, search);

  const [activeFor, setActiveFor] = useState("");
  const resultKey = `${open}|${search}|${entries.map((entry) => entry.id).join(",")}`;
  if (activeFor !== resultKey) {
    setActiveFor(resultKey);
    setActive(0);
  }

  const optionId = (index: number) => `${listId}-option-${index}`;

  useEffect(() => {
    const list = listRef.current;
    if (!open || list === null) return;
    const option = list.querySelector<HTMLElement>(`[data-index="${active}"]`);
    if (option === null) return;
    const box = option.getBoundingClientRect();
    const view = list.getBoundingClientRect();
    if (box.top < view.top) list.scrollTop -= view.top - box.top;
    else if (box.bottom > view.bottom) list.scrollTop += box.bottom - view.bottom;
  }, [active, open]);

  function close(next: boolean) {
    if (!next) setQuery("");
    onOpenChange(next);
  }

  function choose(entry: PaletteEntry | undefined) {
    if (!entry) return;
    close(false);
    void navigate(entry.to);
  }

  function onKeyDown(event: React.KeyboardEvent) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActive((i) => (entries.length === 0 ? 0 : (i + 1) % entries.length));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActive((i) =>
        entries.length === 0 ? 0 : (i - 1 + entries.length) % entries.length,
      );
    } else if (event.key === "Enter") {
      event.preventDefault();
      choose(entries[active]);
    }
  }

  const grouped = new Map<PaletteGroup, { entry: PaletteEntry; index: number }[]>();
  entries.forEach((entry, index) => {
    const rows = grouped.get(entry.group) ?? [];
    rows.push({ entry, index });
    grouped.set(entry.group, rows);
  });
  const groups = PALETTE_GROUPS.filter((group) => grouped.has(group));
  const nothing = entries.length === 0 && settled && query.trim() === search;

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent
        className="bg-card shadow-float top-[12%] w-[min(560px,calc(100%-24px))] max-w-none translate-y-0 gap-0 overflow-hidden rounded-xl p-0 sm:max-w-none"
        showCloseButton={false}
      >
        <DialogTitle className="sr-only">{t("palette.title")}</DialogTitle>

        <div className="flex h-12.5 items-center gap-2.5 border-b px-3.5">
          <Search className="text-muted-fg size-4.25 shrink-0" aria-hidden="true" />
          <input
            // eslint-disable-next-line jsx-a11y/no-autofocus
            autoFocus
            role="combobox"
            aria-expanded={entries.length > 0}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={
              entries[active] === undefined ? undefined : optionId(active)
            }
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={onKeyDown}
            placeholder={t("palette.placeholder")}
            aria-label={t("palette.placeholder")}
            className="text-md h-auto min-w-0 flex-1 border-0 bg-transparent p-0 outline-none"
          />
          <Kbd className="text-2xs text-muted-fg h-auto rounded-sm border bg-transparent px-1.25 py-0 font-sans leading-normal">
            {t("palette.escape")}
          </Kbd>
        </div>

        <div
          ref={listRef}
          id={listId}
          role="listbox"
          aria-label={t("palette.title")}
          className="max-h-90 overflow-y-auto p-1.5"
        >
          {nothing ? (
            <p role="status" className="text-muted-fg text-ui px-2.5 py-6 text-center">
              {t("palette.noMatches")}
            </p>
          ) : null}
          {groups.map((group) => (
            <div key={group} role="group" aria-labelledby={`${listId}-${group}`}>
              <p
                id={`${listId}-${group}`}
                className="text-muted-fg px-2.5 pt-2 pb-1 text-xs leading-normal"
              >
                {t(`palette.${group}`)}
              </p>
              {grouped.get(group)?.map(({ entry, index }) => (
                <button
                  key={entry.id}
                  type="button"
                  id={optionId(index)}
                  role="option"
                  aria-selected={index === active}
                  data-index={index}
                  tabIndex={-1}
                  onMouseMove={() => setActive(index)}
                  onClick={() => choose(entry)}
                  className={cn(
                    "text-ui flex w-full items-center gap-2.5 rounded-[7px] px-2.5 py-2.25 text-left leading-[normal]",
                    index === active && "bg-muted",
                  )}
                >
                  <entry.Icon
                    className="text-muted-fg size-4 shrink-0"
                    aria-hidden="true"
                  />
                  <span className="min-w-0 flex-1 truncate">{entry.label}</span>
                  {entry.hint === "" ? null : (
                    <span className="text-muted-fg max-w-[45%] shrink-0 truncate text-xs">
                      {entry.hint}
                    </span>
                  )}
                </button>
              ))}
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
