import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import {
  AudioLines,
  ClipboardList,
  FileText,
  GraduationCap,
  Headphones,
  LayoutDashboard,
  Library,
  Search,
  Settings,
  Users,
} from "lucide-react";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Kbd } from "@/components/ui/kbd";
import { listQuestions } from "@/features/question-bank/api";
import { listTests } from "@/features/tests/api";
import { useDebounced } from "@/lib/useDebounced";
import { cn } from "@/lib/utils";

interface Entry {
  id: string;
  group: string;
  label: string;
  hint?: string;
  status?: "published" | "draft" | "archived";
  // Carried per entry rather than derived at render.
  Icon: typeof FileText;
  to: string;
}

const DESTINATIONS: { key: string; to: string; Icon: typeof FileText }[] = [
  { key: "nav.dashboard", to: "/admin", Icon: LayoutDashboard },
  { key: "nav.tests", to: "/admin/tests", Icon: FileText },
  { key: "nav.questionBank", to: "/admin/question-bank", Icon: Library },
  { key: "nav.media", to: "/admin/media", Icon: AudioLines },
  { key: "nav.assignments", to: "/admin/assignments", Icon: ClipboardList },
  { key: "nav.students", to: "/admin/students", Icon: Users },
  { key: "nav.classes", to: "/admin/classes", Icon: GraduationCap },
  { key: "nav.settings", to: "/admin/settings", Icon: Settings },
];

/**
 * A-02: one component, three jobs — navigation, search across tests and
 * questions, and getting to a screen from anywhere.
 */
export function CommandPalette({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);
  const search = useDebounced(query.trim(), 250);
  const searching = search !== "";

  const tests = useQuery({
    queryKey: ["palette-tests", search],
    queryFn: ({ signal }) => listTests({ q: search, limit: 5 }, signal),
    enabled: open && searching,
  });
  const questions = useQuery({
    queryKey: ["palette-questions", search],
    queryFn: ({ signal }) => listQuestions({ q: search, limit: 5 }, signal),
    enabled: open && searching,
  });

  const entries: Entry[] = [
    ...(tests.data?.items ?? []).map((test) => ({
      id: `test-${test.id}`,
      group: t("palette.tests"),
      label: test.title,
      status: test.status,
      Icon: FileText,
      to: `/admin/tests/${test.id}`,
    })),
    ...(questions.data?.items ?? []).map((question) => ({
      id: `question-${question.id}`,
      group: t("palette.questions"),
      label: question.prompt,
      hint: question.tags.join(", "),
      Icon: question.media?.kind === "audio" ? Headphones : Library,
      to: `/admin/question-bank/${question.id}`,
    })),
    ...DESTINATIONS.filter((d) => !searching || matches(t(d.key), search)).map((d) => ({
      id: `nav-${d.to}`,
      group: t("palette.navigate"),
      label: t(d.key),
      Icon: d.Icon,
      to: d.to,
    })),
  ];

  // The highlight has to land somewhere real after the results change under it.
  const [activeFor, setActiveFor] = useState("");
  const resultKey = `${search}|${entries.length}`;
  if (activeFor !== resultKey) {
    setActiveFor(resultKey);
    setActive(0);
  }

  function close(next: boolean) {
    if (!next) setQuery("");
    onOpenChange(next);
  }

  function choose(entry: Entry | undefined) {
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

  let lastGroup = "";

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent
        className="max-w-lg gap-0 overflow-hidden p-0"
        showCloseButton={false}
      >
        <DialogTitle className="sr-only">{t("palette.title")}</DialogTitle>

        <div className="flex h-12 items-center gap-2.5 border-b px-3.5">
          <Search
            className="text-muted-foreground size-4 shrink-0"
            aria-hidden="true"
          />
          <input
            // eslint-disable-next-line jsx-a11y/no-autofocus
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={onKeyDown}
            placeholder={t("palette.placeholder")}
            aria-label={t("palette.placeholder")}
            className="h-auto flex-1 border-0 bg-transparent p-0 text-sm outline-none"
          />
          <Kbd>{t("palette.escape")}</Kbd>
        </div>

        <div ref={listRef} className="max-h-80 overflow-y-auto p-1.5">
          {entries.length === 0 ? (
            <p className="text-muted-foreground px-2.5 py-6 text-center text-sm">
              {searching ? t("palette.noMatches") : t("palette.hint")}
            </p>
          ) : (
            entries.map((entry, index) => {
              const heading = entry.group !== lastGroup ? entry.group : null;
              lastGroup = entry.group;
              return (
                <div key={entry.id}>
                  {heading ? (
                    <p className="text-muted-foreground px-3 pt-2 pb-1 text-[0.6875rem] font-semibold tracking-[0.06em] uppercase">
                      {heading}
                    </p>
                  ) : null}
                  <button
                    type="button"
                    onMouseEnter={() => setActive(index)}
                    onClick={() => choose(entry)}
                    className={cn(
                      "flex w-full items-center gap-2 rounded-sm px-2.5 py-1.5 text-left text-[0.8125rem]",
                      index === active ? "bg-accent" : "",
                    )}
                  >
                    <entry.Icon
                      className="text-muted-foreground size-4 shrink-0"
                      aria-hidden="true"
                    />
                    <span className="truncate">{entry.label}</span>
                    {entry.status ? (
                      <StatusBadge
                        className="ml-auto"
                        kind="test"
                        status={entry.status}
                      />
                    ) : null}
                    {entry.hint ? (
                      <span className="text-muted-foreground ml-auto shrink-0 text-xs">
                        {entry.hint}
                      </span>
                    ) : null}
                  </button>
                </div>
              );
            })
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

/** Accent-insensitive on the client too, so the navigation group behaves like search. */
function matches(label: string, query: string): boolean {
  const fold = (s: string) =>
    s.toLocaleLowerCase("vi").normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d");
  return fold(label).includes(fold(query));
}
