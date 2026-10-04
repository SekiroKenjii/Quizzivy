import type { ReactElement, ReactNode } from "react";
import { MemoryRouter } from "react-router";
import {
  ArrowUpRight,
  ChartColumn,
  CircleDot,
  CircleStop,
  Clock,
  Copy,
  Headphones,
  Pencil,
  X,
  type LucideIcon,
} from "lucide-react";
import { DataTable, type DataColumn } from "@/components/shared/data/DataTable";
import { Avatar } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useBulkSelection } from "@/hooks/useBulkSelection";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import {
  registerContentElement,
  useContentBand,
  useContentWidthAtLeast,
} from "@/layouts/shell/contentWidth";
import { cn } from "@/lib/utils";

interface Assignment {
  id: string;
  title: string;
  classes: string;
  when: string;
  meta: string;
  submitted: number;
  target: number;
  toGrade: number;
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
    toGrade: 6,
  },
  {
    id: "a2",
    title: "Unit 4 · Listening: Directions",
    classes: "IELTS Foundation A",
    when: "Tomorrow, 08:00",
    meta: "Test v1 · 20 questions",
    submitted: 9,
    target: 16,
    toGrade: 0,
  },
  {
    id: "a3",
    title: "Vocabulary Quiz: Travel",
    classes: "TOEIC 600 Weekend, Kids Starters B",
    when: "Fri 26 Sep",
    meta: "Test v2 · 25 questions",
    submitted: 11,
    target: 30,
    toGrade: 0,
  },
];

function share(row: Assignment) {
  return row.target === 0 ? 0 : Math.round((row.submitted / row.target) * 100);
}

function bar(row: Assignment): ReactNode {
  return (
    <span className="bg-muted h-1.5 flex-1 overflow-hidden rounded-sm">
      <span className="bg-brand block h-full" style={{ width: `${share(row)}%` }} />
    </span>
  );
}

function pill(label: string): ReactNode {
  return (
    <Badge variant="success" dot className="in-data-[scale=deck]:px-2">
      {label}
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
        <span className="text-muted-fg text-meta flex gap-2 leading-normal">
          {row.meta}
          {row.toGrade > 0 && (
            <span className="text-warning-ink font-medium">
              · {row.toGrade} to grade
            </span>
          )}
        </span>
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
    cell: () => pill("Live"),
  },
];

function assignmentCard(row: Assignment): ReactNode {
  return (
    <>
      <span className="flex items-start justify-between gap-2.5">
        <span className="text-base leading-[normal] font-medium">{row.title}</span>
        {pill("Live")}
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

function assignmentMenu(): ReactNode {
  return (
    <>
      <DropdownMenuItem>
        <ArrowUpRight aria-hidden="true" />
        Open
      </DropdownMenuItem>
      <DropdownMenuItem>
        <Pencil aria-hidden="true" />
        Edit settings
      </DropdownMenuItem>
      <DropdownMenuItem>
        <Clock aria-hidden="true" />
        Extend deadline
      </DropdownMenuItem>
      <DropdownMenuItem>
        <Copy aria-hidden="true" />
        Duplicate
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem variant="destructive">
        <CircleStop aria-hidden="true" />
        Close early
      </DropdownMenuItem>
    </>
  );
}

interface Student {
  id: string;
  name: string;
  user: string;
  classes: readonly string[];
  average: string;
  last: string;
}

const STUDENTS: readonly Student[] = [
  {
    id: "s1",
    name: "Trần Minh Anh",
    user: "@minhanh.tran",
    classes: ["IELTS 6.5 Evening"],
    average: "88%",
    last: "5 min ago",
  },
  {
    id: "s2",
    name: "Lê Hoàng Nam",
    user: "@nam.le",
    classes: ["IELTS 6.5 Evening"],
    average: "71%",
    last: "12 min ago",
  },
];

const STUDENT_COLUMNS: readonly DataColumn<Student>[] = [
  {
    id: "student",
    header: "Student",
    track: "minmax(200px,2fr)",
    cell: (row, shown) => (
      <span className="flex min-w-0 items-center gap-2.5">
        <Avatar name={row.name} />
        <span className="min-w-0">
          <span className="block truncate font-medium">{row.name}</span>
          <span className="text-muted-fg block truncate text-xs leading-normal">
            {[
              row.user,
              !shown.has("classes") && row.classes.join(", "),
              !shown.has("last") && row.last,
            ]
              .filter(Boolean)
              .join(" · ")}
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
      <span className="flex min-w-0 flex-wrap gap-1.25">
        {row.classes.map((name) => (
          <span
            key={name}
            className="rounded-sm border px-1.75 py-px text-xs leading-normal whitespace-nowrap"
          >
            {name}
          </span>
        ))}
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
  {
    id: "account",
    header: "Account",
    track: "130px",
    showFrom: 660,
    cell: () => pill("Active"),
  },
];

function studentCard(row: Student): ReactNode {
  return (
    <>
      <Avatar name={row.name} size="36" />
      <span className="min-w-0 flex-1">
        <span className="block font-medium">{row.name}</span>
        <span className="text-muted-fg text-meta block truncate leading-normal">
          {row.classes.join(", ")} · {row.last}
        </span>
      </span>
      <span className="font-semibold tabular-nums">{row.average}</span>
    </>
  );
}

function openNothing() {}

function resultsMenu(): ReactNode {
  return (
    <DropdownMenuItem>
      <ChartColumn aria-hidden="true" />
      View results
    </DropdownMenuItem>
  );
}

interface Member {
  id: string;
  name: string;
  email: string;
  via: string;
  admin: boolean;
  at: string;
  submitted: string;
  average: number;
}

const MEMBERS: readonly Member[] = [
  {
    id: "m1",
    name: "Lê Hoàng Nam",
    email: "nam.le@gmail.com",
    via: "Admin",
    admin: true,
    at: "20 Aug",
    submitted: "5",
    average: 59,
  },
  {
    id: "m2",
    name: "Trần Minh Anh",
    email: "anh.tran@gmail.com",
    via: "Code ••2PXA",
    admin: false,
    at: "9 Sep",
    submitted: "7",
    average: 73,
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
    id: "via",
    header: "Joined via",
    track: "120px",
    cell: (row) => (
      <span
        className={cn(
          "inline-flex h-5.5 items-center rounded-sm px-2 text-xs font-medium whitespace-nowrap",
          row.admin ? "bg-primary text-primary-fg" : "bg-muted font-mono",
        )}
      >
        {row.via}
      </span>
    ),
  },
  {
    id: "at",
    header: "Joined",
    track: "80px",
    cell: (row) => (
      <span className="text-muted-fg block text-sm leading-normal whitespace-nowrap">
        {row.at}
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
    cell: (row) => (
      <span className={cn("tabular-nums", row.average < 60 && "text-danger-ink")}>
        {row.average}%
      </span>
    ),
  },
];

interface Question {
  id: string;
  prompt: string;
  type: string;
  icon: LucideIcon;
  level: string;
  used: number;
  updated: string;
  tags: readonly string[];
}

const QUESTIONS: readonly Question[] = [
  {
    id: "q1",
    prompt: "Choose the correct form: She ___ in Hanoi since 2019.",
    type: "Single choice",
    icon: CircleDot,
    level: "A2",
    used: 4,
    updated: "2 d ago",
    tags: ["grammar", "present perfect"],
  },
  {
    id: "q2",
    prompt: "Listen and label the map: where is the post office?",
    type: "Audio · labelling",
    icon: Headphones,
    level: "B1",
    used: 2,
    updated: "1 w ago",
    tags: ["listening", "map"],
  },
];

const QUESTION_COLUMNS: readonly DataColumn<Question>[] = [
  {
    id: "question",
    header: "Question",
    track: "minmax(220px,3fr)",
    cell: (row, shown) => (
      <span className="block min-w-0">
        <span className="block truncate">{row.prompt}</span>
        {shown.has("type") ? null : (
          <span className="text-muted-fg mt-0.5 flex items-center gap-1.5 text-xs leading-normal">
            <row.icon aria-hidden="true" className="size-3" />
            {row.type} · {row.level}
          </span>
        )}
      </span>
    ),
    aside: (row) => (
      <span className="mt-0.75 flex flex-wrap gap-1.25">
        {row.tags.map((tag) => (
          <span
            key={tag}
            className="bg-muted inline-flex h-5.5 items-center gap-0.5 rounded-full border pr-0.75 pl-2 text-xs leading-none font-medium whitespace-nowrap"
          >
            {tag}
            <button
              type="button"
              aria-label={`Remove tag ${tag}`}
              className="text-muted-fg hover:bg-hover hover:text-fg grid size-4 flex-none cursor-pointer place-items-center rounded-full"
            >
              <X aria-hidden="true" className="size-2.75" />
            </button>
          </span>
        ))}
      </span>
    ),
  },
  {
    id: "type",
    header: "Type",
    track: "140px",
    showFrom: 640,
    cell: (row) => (
      <span className="flex items-center gap-1.75 text-sm leading-normal">
        <row.icon aria-hidden="true" className="text-muted-fg size-3.5" />
        {row.type}
      </span>
    ),
  },
  {
    id: "level",
    header: "Level",
    track: "70px",
    showFrom: 540,
    cell: (row) => <span className="block text-sm leading-normal">{row.level}</span>,
  },
  {
    id: "used",
    header: "Used",
    track: "80px",
    showFrom: 780,
    cell: (row) => (
      <span className="text-muted-fg block text-sm leading-normal tabular-nums">
        {row.used} tests
      </span>
    ),
  },
  {
    id: "updated",
    header: "Updated",
    track: "90px",
    showFrom: 900,
    cell: (row) => (
      <span className="text-muted-fg block text-sm leading-normal">{row.updated}</span>
    ),
  },
];

const LONG_ASSIGNMENTS: readonly Assignment[] = [
  {
    id: "l1",
    title:
      "Bài kiểm tra giữa kỳ kỹ năng Đọc hiểu dành cho lớp luyện thi IELTS 6.5 buổi tối, đợt tháng Mười",
    classes:
      "Luyện thi IELTS 6.5 buổi tối thứ Hai, thứ Tư, thứ Sáu, Tiếng Anh thiếu nhi Starters B",
    when: "Hôm nay, 21:00",
    meta: "Đề v3 · 40 câu hỏi",
    submitted: 18,
    target: 24,
    toGrade: 6,
  },
  {
    id: "l2",
    title: "Nghe",
    classes: "Nền tảng A",
    when: "Thứ Sáu, 26 tháng 9",
    meta: "Đề v1 · 20 câu hỏi",
    submitted: 0,
    target: 0,
    toGrade: 0,
  },
];

const LONG_COLUMNS: readonly DataColumn<Assignment>[] = ASSIGNMENT_COLUMNS.map(
  (column) => ({
    ...column,
    header:
      {
        title: "Bài giao",
        classes: "Giao cho",
        when: "Đóng lúc",
        submitted: "Đã nộp",
        status: "Trạng thái",
      }[column.id] ?? column.header,
  }),
);

const LONG_STUDENTS: readonly Student[] = [
  {
    id: "v1",
    name: "Công Tằng Tôn Nữ Nguyễn Thị Hoàng Bảo Ngọc Phương Anh",
    user: "@congtangtonnu.nguyenthihoangbaongocphuonganh",
    classes: ["Luyện thi IELTS 6.5 buổi tối", "Tiếng Anh thiếu nhi Starters B"],
    average: "100%",
    last: "5 phút trước",
  },
  {
    id: "v2",
    name: "Lê Nam",
    user: "@nam.le",
    classes: ["Nền tảng A"],
    average: "7%",
    last: "Vừa xong",
  },
];

function footerLine(text: string): ReactNode {
  return (
    <div className="text-muted-fg sticky left-0 border-t px-4 py-2.5 text-sm leading-normal">
      {text}
    </div>
  );
}

const MEMBER_BANDS = [520, 620, 720, 1000, 1080] as const;

const MEMBER_SETS: readonly ReadonlySet<string>[] = [
  new Set(["name", "submitted"]),
  new Set(["name", "submitted", "average"]),
  new Set(["name", "via", "submitted", "average"]),
  new Set(["name", "via", "at", "submitted", "average"]),
  new Set(["name", "via", "submitted", "average"]),
  new Set(["name", "via", "at", "submitted", "average"]),
];

export const cases: Record<string, () => ReactElement> = {
  assignments: function AssignmentsTable() {
    const selection = useBulkSelection<Assignment>();
    return (
      <MemoryRouter>
        <div ref={registerContentElement}>
          <DataTable
            label="Assignments"
            columns={ASSIGNMENT_COLUMNS}
            rows={ASSIGNMENTS}
            rowSize={{ minHeight: 60 }}
            rowHref={(row) => `/teacher/assignments/${row.id}`}
            selection={selection}
            rowName={(row) => row.title}
            menu={assignmentMenu}
            card={assignmentCard}
          />
        </div>
      </MemoryRouter>
    );
  },

  students: function StudentsTable() {
    const selection = useBulkSelection<Student>();
    return (
      <div ref={registerContentElement}>
        <DataTable
          label="Students"
          columns={STUDENT_COLUMNS}
          rows={STUDENTS}
          rowSize={{ height: 56 }}
          onOpen={openNothing}
          selection={selection}
          rowName={(row) => row.name}
          menu={resultsMenu}
          card={studentCard}
          cardLayout="joined"
        />
      </div>
    );
  },

  members: function MembersTable() {
    const selection = useBulkSelection<Member>();
    const band = useContentBand(MEMBER_BANDS);
    const two = useContentWidthAtLeast(1000);
    return (
      <div
        ref={registerContentElement}
        className="grid items-start gap-4.5"
        style={{
          gridTemplateColumns: two ? "minmax(0,2fr) minmax(0,1fr)" : "minmax(0,1fr)",
        }}
      >
        <section className="bg-card shadow-card overflow-hidden rounded-xl border">
          <div className="flex min-h-15.5 flex-wrap items-center justify-between gap-3 px-4.5 pt-4 pb-3">
            <h2 className="text-md leading-normal font-semibold tracking-[-0.01em] whitespace-nowrap">
              Members · 24
            </h2>
          </div>
          <DataTable
            label="Members"
            columns={MEMBER_COLUMNS}
            rows={MEMBERS}
            rowSize={{ minHeight: 56 }}
            shown={MEMBER_SETS[band]!}
            selection={selection}
            rowName={(row) => row.name}
            menu={resultsMenu}
            menuTrack="32px"
            padX={18}
            framed={false}
          />
        </section>
      </div>
    );
  },

  bank: function BankTable() {
    const selection = useBulkSelection<Question>();
    const wide = useMediaQuery("(min-width: 1024px)");
    return (
      <div ref={registerContentElement} className="flex items-start gap-3.5">
        {wide ? <div className="w-55 flex-none" /> : null}
        <div className="min-w-0 flex-1">
          <DataTable
            label="Questions"
            columns={QUESTION_COLUMNS}
            rows={QUESTIONS}
            rowSize={{ padY: 10 }}
            onOpen={openNothing}
            selection={selection}
            rowName={(row) => row.prompt}
          />
        </div>
      </div>
    );
  },

  "long-vi": function LongVietnamese() {
    const assignments = useBulkSelection<Assignment>();
    const students = useBulkSelection<Student>();
    return (
      <MemoryRouter>
        <div ref={registerContentElement} className="flex flex-col gap-4">
          <DataTable
            label="Bài giao"
            columns={LONG_COLUMNS}
            rows={LONG_ASSIGNMENTS}
            rowSize={{ minHeight: 60 }}
            rowHref={(row) => `/teacher/assignments/${row.id}`}
            selection={assignments}
            rowName={(row) => row.title}
            menu={assignmentMenu}
            card={assignmentCard}
            footer={footerLine("1–2 trên 2 bài giao")}
          />
          <DataTable
            label="Học viên"
            columns={STUDENT_COLUMNS}
            rows={LONG_STUDENTS}
            rowSize={{ height: 56 }}
            onOpen={openNothing}
            selection={students}
            rowName={(row) => row.name}
            menu={resultsMenu}
            card={studentCard}
            cardLayout="joined"
            footer={footerLine("1–2 trên 2 học viên")}
          />
        </div>
      </MemoryRouter>
    );
  },

  empty: function EmptyTables() {
    const assignments = useBulkSelection<Assignment>();
    const students = useBulkSelection<Student>();
    return (
      <MemoryRouter>
        <div ref={registerContentElement} className="flex flex-col gap-4">
          <DataTable
            label="Bài giao"
            columns={LONG_COLUMNS}
            rows={[]}
            rowSize={{ minHeight: 60 }}
            rowHref={(row) => `/teacher/assignments/${row.id}`}
            selection={assignments}
            rowName={(row) => row.title}
            menu={assignmentMenu}
            card={assignmentCard}
            empty="Chưa có bài giao nào ở đây."
            footer={footerLine("Không có bài giao nào")}
          />
          <DataTable
            label="Học viên"
            columns={STUDENT_COLUMNS}
            rows={[]}
            rowSize={{ height: 56 }}
            onOpen={openNothing}
            selection={students}
            rowName={(row) => row.name}
            menu={resultsMenu}
            card={studentCard}
            cardLayout="joined"
            empty="Chưa có học viên nào khớp với bộ lọc này."
            footer={footerLine("Không có học viên nào")}
          />
        </div>
      </MemoryRouter>
    );
  },

  dense: function DenseTables() {
    const assignments = useBulkSelection<Assignment>();
    const students = useBulkSelection<Student>();
    const questions = useBulkSelection<Question>();
    return (
      <MemoryRouter>
        <div ref={registerContentElement} className="flex flex-col gap-4">
          <DataTable
            label="Bài giao"
            columns={LONG_COLUMNS}
            rows={LONG_ASSIGNMENTS}
            rowSize={{ minHeight: 60 }}
            dense
            rowHref={(row) => `/teacher/assignments/${row.id}`}
            selection={assignments}
            rowName={(row) => row.title}
            menu={assignmentMenu}
          />
          <DataTable
            label="Học viên"
            columns={STUDENT_COLUMNS}
            rows={LONG_STUDENTS}
            rowSize={{ height: 56 }}
            dense
            selection={students}
            rowName={(row) => row.name}
            menu={resultsMenu}
          />
          <DataTable
            label="Câu hỏi"
            columns={QUESTION_COLUMNS}
            rows={QUESTIONS}
            rowSize={{ padY: 10 }}
            dense
            onOpen={openNothing}
            selection={questions}
            rowName={(row) => row.prompt}
          />
        </div>
      </MemoryRouter>
    );
  },

  primitive: () => (
    <div className="bg-card shadow-card overflow-hidden rounded-xl border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Question</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Level</TableHead>
            <TableHead>Used</TableHead>
            <TableHead>Updated</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {QUESTIONS.map((row) => (
            <TableRow key={row.id}>
              <TableCell>{row.prompt}</TableCell>
              <TableCell>{row.type}</TableCell>
              <TableCell>{row.level}</TableCell>
              <TableCell>{row.used} tests</TableCell>
              <TableCell>{row.updated}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  ),
};
