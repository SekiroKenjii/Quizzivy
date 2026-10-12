import { useState, type ReactElement, type ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { CircleStop, Clock, Download, Flag, UserMinus } from "lucide-react";
import { BulkActions, BulkBarButton } from "@/components/shared/BulkActions";
import type { BulkSelection } from "@/components/shared/BulkSelection";
import { DataTable, type DataColumn } from "@/components/shared/data/DataTable";
import { Pager } from "@/components/shared/data/Pager";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { usePage, usePageSize } from "@/hooks/usePage";
import { registerContentElement } from "@/layouts/shell/contentWidth";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";

type Words = readonly [one: string, many: string, vi: string];

const ASSIGNMENT_WORDS: Words = ["assignment", "assignments", "bài giao"];
const STUDENT_WORDS: Words = ["student", "students", "học viên"];
const QUESTION_WORDS: Words = ["question", "questions", "câu hỏi"];

function useText() {
  const english = useLocale() === "en";
  return (en: string, vi: string) => (english ? en : vi);
}

function useNoun([one, many, vi]: Words) {
  const english = useLocale() === "en";
  return (count: number) => {
    if (!english) return vi;
    return count === 1 ? one : many;
  };
}

const parts = {
  UrlPager: function UrlPager({
    total,
    words,
    frame,
  }: Readonly<{
    total: number;
    words: Words;
    frame?: "card" | "plain" | undefined;
  }>) {
    const [page] = usePage();
    const [size] = usePageSize();
    const noun = useNoun(words);
    return (
      <Pager page={page} pageSize={size} total={total} noun={noun} frame={frame} />
    );
  },
};

function useSeeded<T extends { id: string }>(seed: readonly T[]): BulkSelection<T> {
  const [selected, setSelected] = useState<ReadonlyMap<string, T>>(
    () => new Map(seed.map((item) => [item.id, item])),
  );
  return {
    selected,
    toggle: (item) =>
      setSelected((current) => {
        const next = new Map(current);
        if (!next.delete(item.id)) next.set(item.id, item);
        return next;
      }),
    selectPage: (items, checked) =>
      setSelected((current) => {
        const next = new Map(current);
        for (const item of items) {
          if (checked) next.set(item.id, item);
          else next.delete(item.id);
        }
        return next;
      }),
    remove: (ids) =>
      setSelected((current) => {
        const next = new Map(current);
        for (const id of ids) next.delete(id);
        return next;
      }),
    clear: () => setSelected(new Map()),
  };
}

function openNothing() {}

function settled() {
  return Promise.resolve();
}

const CARD = "bg-card shadow-card overflow-hidden rounded-xl border";

interface Attempt {
  id: string;
  name: string;
  status: "submitted" | "grading";
  score: string;
  time: string;
  focus: number;
  flagged: boolean;
  at: string;
}

const ROSTER: readonly Attempt[] = [
  {
    id: "r1",
    name: "Lê Hoàng Nam",
    status: "grading",
    score: "31 / 40*",
    time: "52 min",
    focus: 3,
    flagged: true,
    at: "20:14",
  },
  {
    id: "r2",
    name: "Trần Minh Anh",
    status: "submitted",
    score: "36 / 40",
    time: "47 min",
    focus: 0,
    flagged: false,
    at: "20:02",
  },
];

const ATTEMPT_STATUS = {
  submitted: { label: "Submitted", variant: "success", ink: "text-success-ink" },
  grading: { label: "Needs grading", variant: "warning", ink: "text-warning-ink" },
} as const;

const ROSTER_COLUMNS: readonly DataColumn<Attempt>[] = [
  {
    id: "student",
    header: "Student",
    track: "minmax(180px,2fr)",
    cell: (row, shown) => (
      <span className="flex min-w-0 items-center gap-2.5">
        <Avatar name={row.name} size="30" />
        <span className="min-w-0">
          <span className="block truncate font-medium">{row.name}</span>
          {shown.has("status") ? null : (
            <span
              className={cn(
                "flex items-center gap-1.25 text-xs leading-normal",
                ATTEMPT_STATUS[row.status].ink,
              )}
            >
              <span className="size-1.5 flex-none rounded-full bg-current" />
              {ATTEMPT_STATUS[row.status].label}
              {row.flagged ? (
                <span className="text-danger-ink">· {row.focus}× focus lost</span>
              ) : null}
            </span>
          )}
        </span>
      </span>
    ),
  },
  {
    id: "status",
    header: "Status",
    track: "130px",
    showFrom: 560,
    cell: (row) => (
      <Badge
        variant={ATTEMPT_STATUS[row.status].variant}
        dot
        className="in-data-[scale=deck]:px-2"
      >
        {ATTEMPT_STATUS[row.status].label}
      </Badge>
    ),
  },
  {
    id: "score",
    header: "Score",
    track: "90px",
    cell: (row) => <span className="font-medium tabular-nums">{row.score}</span>,
  },
  {
    id: "time",
    header: "Time",
    track: "80px",
    showFrom: 820,
    cell: (row) => <span className="text-muted-fg tabular-nums">{row.time}</span>,
  },
  {
    id: "focus",
    header: "Focus lost",
    track: "100px",
    showFrom: 700,
    cell: (row) => (
      <span
        className={cn(
          "flex items-center gap-1.25 tabular-nums",
          row.flagged ? "text-danger-ink" : "text-muted-fg",
        )}
      >
        {row.flagged ? <Flag aria-hidden="true" className="size-3.25" /> : null}
        {row.focus > 0 ? `${row.focus}×` : "—"}
      </span>
    ),
  },
  {
    id: "at",
    header: "Submitted",
    track: "100px",
    showFrom: 940,
    cell: (row) => <span className="text-muted-fg">{row.at}</span>,
  },
];

interface Assignment {
  id: string;
  title: string;
  classes: string;
  when: string;
  meta: string;
  submitted: number;
  target: number;
}

const ASSIGNMENTS: readonly Assignment[] = [
  {
    id: "a1",
    title: "Mid-term Reading Mock",
    classes: "IELTS 6.5 Evening",
    when: "Today, 21:00",
    meta: "Test v3 · 40 questions",
    submitted: 18,
    target: 24,
  },
  {
    id: "a2",
    title: "Unit 4 · Listening: Directions",
    classes: "IELTS Foundation A",
    when: "Tomorrow, 08:00",
    meta: "Test v1 · 20 questions",
    submitted: 9,
    target: 16,
  },
  {
    id: "a3",
    title: "Vocabulary Quiz: Travel",
    classes: "TOEIC 600 Weekend, Kids Starters B",
    when: "Fri 26 Sep",
    meta: "Test v2 · 25 questions",
    submitted: 11,
    target: 30,
  },
];

function bar(row: Assignment): ReactNode {
  return (
    <span className="bg-muted h-1.5 flex-1 overflow-hidden rounded-sm">
      <span
        className="bg-brand block h-full"
        style={{ width: `${Math.round((row.submitted / row.target) * 100)}%` }}
      />
    </span>
  );
}

function live(): ReactNode {
  return (
    <Badge variant="success" dot className="in-data-[scale=deck]:px-2">
      Live
    </Badge>
  );
}

const ASSIGNMENT_COLUMNS: readonly DataColumn<Assignment>[] = [
  {
    id: "title",
    header: "Assignment",
    track: "minmax(200px,2.2fr)",
    cell: (row, shown) => (
      <span className="block min-w-0">
        <span className="block truncate font-medium">{row.title}</span>
        <span className="text-muted-fg text-meta block leading-normal">{row.meta}</span>
        {shown.has("classes") && shown.has("when") ? null : (
          <span className="text-muted-fg text-meta block truncate leading-normal">
            {row.classes} · {row.when}
          </span>
        )}
      </span>
    ),
  },
  {
    id: "classes",
    header: "Assigned to",
    track: "minmax(130px,1.3fr)",
    showFrom: 860,
    cell: (row) => (
      <span className="text-muted-fg block truncate text-sm leading-normal">
        {row.classes}
      </span>
    ),
  },
  {
    id: "when",
    header: "Closes",
    track: "120px",
    showFrom: 700,
    cell: (row) => <span className="block text-sm leading-normal">{row.when}</span>,
  },
  {
    id: "submitted",
    header: "Submitted",
    track: "150px",
    showFrom: 560,
    cell: (row) => (
      <span className="flex items-center gap-2.5">
        {bar(row)}
        <span className="text-muted-fg text-meta w-10 text-right leading-normal tabular-nums">
          {row.submitted}/{row.target}
        </span>
      </span>
    ),
  },
  {
    id: "status",
    header: "Status",
    track: "110px",
    showFrom: 760,
    cell: live,
  },
];

function assignmentCard(row: Assignment): ReactNode {
  return (
    <>
      <span className="flex items-start justify-between gap-2.5">
        <span className="text-base leading-[normal] font-medium">{row.title}</span>
        {live()}
      </span>
      <span className="text-muted-fg text-meta leading-[normal]">
        {row.classes} · {row.when}
      </span>
      <span className="flex items-center gap-2.5">
        {bar(row)}
        <span className="text-muted-fg text-meta leading-[normal]">
          {row.submitted}/{row.target}
        </span>
      </span>
    </>
  );
}

interface Student {
  id: string;
  name: string;
  user: string;
  classes: string;
  average: string;
  last: string;
}

const STUDENTS: readonly Student[] = [
  {
    id: "s1",
    name: "Trần Minh Anh",
    user: "@minhanh.tran",
    classes: "IELTS 6.5 Evening",
    average: "88%",
    last: "5 min ago",
  },
  {
    id: "s2",
    name: "Lê Hoàng Nam",
    user: "@nam.le",
    classes: "IELTS 6.5 Evening",
    average: "71%",
    last: "12 min ago",
  },
];

const STUDENT_COLUMNS: readonly DataColumn<Student>[] = [
  {
    id: "student",
    header: "Student",
    track: "minmax(200px,2fr)",
    cell: (row) => (
      <span className="flex min-w-0 items-center gap-2.5">
        <Avatar name={row.name} />
        <span className="min-w-0">
          <span className="block truncate font-medium">{row.name}</span>
          <span className="text-muted-fg block truncate text-xs leading-normal">
            {row.user}
          </span>
        </span>
      </span>
    ),
  },
  {
    id: "classes",
    header: "Classes",
    track: "minmax(150px,1.6fr)",
    showFrom: 780,
    cell: (row) => (
      <span className="rounded-sm border px-1.75 py-px text-xs leading-normal whitespace-nowrap">
        {row.classes}
      </span>
    ),
  },
  {
    id: "average",
    header: "Average",
    track: "80px",
    cell: (row) => <span className="font-medium tabular-nums">{row.average}</span>,
  },
  {
    id: "last",
    header: "Last active",
    track: "110px",
    showFrom: 920,
    cell: (row) => (
      <span className="text-muted-fg block text-sm leading-normal">{row.last}</span>
    ),
  },
];

function studentCard(row: Student): ReactNode {
  return (
    <>
      <Avatar name={row.name} size="36" />
      <span className="min-w-0 flex-1">
        <span className="block font-medium">{row.name}</span>
        <span className="text-muted-fg text-meta block truncate leading-normal">
          {row.classes} · {row.last}
        </span>
      </span>
      <span className="font-semibold tabular-nums">{row.average}</span>
    </>
  );
}

interface Member {
  id: string;
  name: string;
  email: string;
  submitted: string;
  average: string;
}

const MEMBERS: readonly Member[] = [
  {
    id: "m1",
    name: "Lê Hoàng Nam",
    email: "nam.le@gmail.com",
    submitted: "5",
    average: "59%",
  },
  {
    id: "m2",
    name: "Trần Minh Anh",
    email: "anh.tran@gmail.com",
    submitted: "7",
    average: "73%",
  },
];

const MEMBER_COLUMNS: readonly DataColumn<Member>[] = [
  {
    id: "name",
    header: "Name",
    track: "minmax(180px,2fr)",
    cell: (row) => (
      <span className="flex min-w-0 items-center gap-2.5">
        <Avatar name={row.name} size="30" />
        <span className="min-w-0">
          <span className="block truncate font-medium">{row.name}</span>
          <span className="text-muted-fg block truncate text-xs leading-normal">
            {row.email}
          </span>
        </span>
      </span>
    ),
  },
  {
    id: "submitted",
    header: "Submitted",
    track: "80px",
    align: "end",
    cell: (row) => <span className="tabular-nums">{row.submitted}</span>,
  },
  {
    id: "average",
    header: "Average",
    track: "70px",
    align: "end",
    showFrom: 520,
    cell: (row) => <span className="tabular-nums">{row.average}</span>,
  },
];

const WIDE_COLUMNS: readonly DataColumn<Attempt>[] = [
  ...ROSTER_COLUMNS.map((column) => ({
    id: column.id,
    header: column.header,
    track: "260px",
    cell: column.cell,
  })),
  {
    id: "note",
    header: "Note",
    track: "420px",
    cell: () => <span className="text-muted-fg">—</span>,
  },
];

const WIDE_SHOWN: ReadonlySet<string> = new Set(
  WIDE_COLUMNS.map((column) => column.id),
);

export const cases: Record<string, () => ReactElement> = {
  "pager-roster": () => (
    <MemoryRouter>
      <div ref={registerContentElement}>
        <DataTable
          label="Students"
          columns={ROSTER_COLUMNS}
          rows={ROSTER}
          rowSize={{ height: 54 }}
          onOpen={openNothing}
          footer={<parts.UrlPager total={24} words={STUDENT_WORDS} />}
        />
      </div>
    </MemoryRouter>
  ),

  "pager-bank": () => (
    <div className="flex flex-col gap-4">
      <MemoryRouter>
        <div className={CARD}>
          <parts.UrlPager total={1284} words={QUESTION_WORDS} />
        </div>
      </MemoryRouter>
      <MemoryRouter initialEntries={["/?page=2"]}>
        <div className={CARD}>
          <parts.UrlPager total={1284} words={QUESTION_WORDS} />
        </div>
      </MemoryRouter>
    </div>
  ),

  "pager-one-page": () => (
    <MemoryRouter>
      <div className={CARD}>
        <parts.UrlPager total={3} words={ASSIGNMENT_WORDS} />
      </div>
    </MemoryRouter>
  ),

  "pager-empty": () => (
    <MemoryRouter>
      <div className={CARD}>
        <parts.UrlPager total={0} words={ASSIGNMENT_WORDS} />
      </div>
    </MemoryRouter>
  ),

  "pager-plain": function PlainPager() {
    const wide = useMediaQuery("(min-width: 768px)");
    return (
      <MemoryRouter>
        <div ref={registerContentElement}>
          <DataTable
            label="Assignments"
            columns={ASSIGNMENT_COLUMNS}
            rows={ASSIGNMENTS}
            rowSize={{ minHeight: 60 }}
            rowHref={(row) => `/teacher/assignments/${row.id}`}
            card={assignmentCard}
            footer={
              <parts.UrlPager
                total={3}
                words={ASSIGNMENT_WORDS}
                frame={wide ? "card" : "plain"}
              />
            }
          />
        </div>
      </MemoryRouter>
    );
  },

  "pager-joined": () => (
    <MemoryRouter>
      <div ref={registerContentElement}>
        <DataTable
          label="Students"
          columns={STUDENT_COLUMNS}
          rows={STUDENTS}
          rowSize={{ height: 56 }}
          card={studentCard}
          cardLayout="joined"
          footer={<parts.UrlPager total={86} words={STUDENT_WORDS} />}
        />
      </div>
    </MemoryRouter>
  ),

  "pager-wide-table": () => (
    <MemoryRouter>
      <DataTable
        label="Students"
        columns={WIDE_COLUMNS}
        rows={ROSTER}
        rowSize={{ height: 54 }}
        shown={WIDE_SHOWN}
        footer={<parts.UrlPager total={24} words={STUDENT_WORDS} />}
      />
    </MemoryRouter>
  ),

  "bulk-bar": function AssignmentsBulkBar() {
    const text = useText();
    const selection = useSeeded(ASSIGNMENTS.slice(0, 2));
    return (
      <MemoryRouter>
        <div ref={registerContentElement} className="flex flex-col gap-4">
          <BulkActions
            hideOnPhone
            selected={[...selection.selected.values()]}
            name={(row) => row.title}
            actions={[
              {
                label: text("Close now", "Đóng ngay"),
                description: text(
                  "Students who have not started can no longer start. Attempts in progress are submitted as they are.",
                  "Học viên chưa bắt đầu sẽ không thể bắt đầu nữa. Bài đang làm được nộp như hiện có.",
                ),
                icon: CircleStop,
                run: settled,
              },
            ]}
            onRemoved={selection.remove}
            onClear={selection.clear}
            onSettled={settled}
          >
            <BulkBarButton icon={Clock} onClick={openNothing}>
              {text("Extend deadline", "Gia hạn")}
            </BulkBarButton>
            <BulkBarButton icon={Download} onClick={openNothing}>
              {text("Export results", "Xuất kết quả")}
            </BulkBarButton>
          </BulkActions>
          <DataTable
            label="Assignments"
            columns={ASSIGNMENT_COLUMNS}
            rows={ASSIGNMENTS}
            rowSize={{ minHeight: 60 }}
            rowHref={(row) => `/teacher/assignments/${row.id}`}
            selection={selection}
            rowName={(row) => row.title}
            card={assignmentCard}
          />
        </div>
      </MemoryRouter>
    );
  },

  "bulk-bar-members": function MembersBulkBar() {
    const text = useText();
    const selection = useSeeded(MEMBERS.slice(0, 1));
    return (
      <MemoryRouter>
        <section ref={registerContentElement} className={CARD}>
          <div className="flex min-h-15.5 flex-wrap items-center justify-between gap-3 px-4.5 pt-4 pb-3">
            <h2 className="text-md leading-normal font-semibold tracking-[-0.01em] whitespace-nowrap">
              {text("Members · 24", "Thành viên · 24")}
            </h2>
          </div>
          <BulkActions
            className="mx-3 mb-2.5"
            selected={[...selection.selected.values()]}
            name={(row) => row.name}
            actions={[
              {
                label: text("Remove selected", "Xoá khỏi lớp"),
                description: text(
                  "Their accounts and results stay.",
                  "Tài khoản và kết quả của các em vẫn được giữ.",
                ),
                icon: UserMinus,
                run: settled,
              },
            ]}
            onRemoved={selection.remove}
            onClear={selection.clear}
            onSettled={settled}
          />
          <DataTable
            label="Members"
            columns={MEMBER_COLUMNS}
            rows={MEMBERS}
            rowSize={{ minHeight: 56 }}
            selection={selection}
            rowName={(row) => row.name}
            padX={18}
            framed={false}
          />
        </section>
      </MemoryRouter>
    );
  },

  "bulk-bar-long-vi": function LongBulkBar() {
    const selection = useSeeded(ASSIGNMENTS);
    return (
      <MemoryRouter>
        <BulkActions
          selected={[...selection.selected.values()]}
          selectionLabel="Đã chọn 3 bài giao trên 2 trang"
          name={(row) => row.title}
          actions={[
            {
              label: "Đặt lại mật khẩu cho học viên",
              description: "Mỗi học viên nhận một mật khẩu tạm thời.",
              destructive: false,
              run: settled,
            },
            {
              label: "Đóng ngay tất cả bài giao đã chọn",
              description: "Bài đang làm được nộp như hiện có.",
              icon: CircleStop,
              run: settled,
            },
          ]}
          onRemoved={selection.remove}
          onClear={selection.clear}
          onSettled={settled}
        >
          <BulkBarButton icon={Clock} onClick={openNothing}>
            Gia hạn thời gian làm bài
          </BulkBarButton>
          <BulkBarButton icon={Download} onClick={openNothing} disabled>
            Xuất kết quả
          </BulkBarButton>
        </BulkActions>
      </MemoryRouter>
    );
  },
};
