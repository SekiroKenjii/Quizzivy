import { useState, type ReactElement } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import {
  ArrowLeftRight,
  ArrowUpRight,
  ChevronDown,
  CircleCheck,
  CircleDot,
  CircleStop,
  ClipboardPaste,
  Clock,
  Copy,
  FileCode,
  FileUp,
  Pencil,
  SquareCheck,
  TextAlignStart,
  TextCursorInput,
  Type,
  X,
} from "lucide-react";
import { RowMenu } from "@/components/shared/RowMenu";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemText,
  DropdownMenuLabel,
  DropdownMenuMeta,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Segmented } from "@/components/ui/segmented";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { AccountMenu } from "@/features/auth/AccountMenu";
import { useAuthStore } from "@/stores/auth";

const CLASSES = [
  "IELTS 6.5 Evening",
  "IELTS Foundation A",
  "TOEIC 600 Weekend",
  "Kids Starters B",
];

const STUDENT = {
  id: "018f0000-0000-7000-8000-0000000000b1",
  email: "giabao.ng@gmail.com",
  fullName: "Nguyễn Gia Bảo",
  role: "student" as const,
  hasPassword: true,
  linkedProviders: [],
  mustChangePassword: false,
  createdAt: "2026-01-01T00:00:00Z",
  permissions: ["learning.take_tests" as const],
  workspaces: ["app" as const],
};

const QUESTION_TYPES = [
  { label: "Single choice", icon: CircleDot },
  { label: "Multiple choice", icon: SquareCheck },
  { label: "True / False", icon: CircleCheck },
  { label: "Fill in the blank", icon: TextCursorInput },
  { label: "Short answer", icon: TextAlignStart },
  { label: "Matching", icon: ArrowLeftRight },
];

const LONG_STATUSES = [
  { value: "live", label: "Đang mở cho học viên làm bài", count: 12 },
  { value: "scheduled", label: "Đã lên lịch", count: 0 },
  { value: "closed", label: "Đã đóng và đã chấm xong", count: 120 },
  { value: "draft", label: "Bản nháp", count: 3 },
];

function toggled(chosen: readonly string[], name: string) {
  return chosen.includes(name)
    ? chosen.filter((other) => other !== name)
    : [...chosen, name];
}

function assignmentMenu(draft: boolean): ReactElement {
  return (
    <div className="flex justify-end">
      <RowMenu label="More">
        <DropdownMenuItem>
          <ArrowUpRight aria-hidden="true" />
          <DropdownMenuItemText>Open</DropdownMenuItemText>
        </DropdownMenuItem>
        <DropdownMenuItem>
          <Pencil aria-hidden="true" />
          <DropdownMenuItemText>Edit settings</DropdownMenuItemText>
        </DropdownMenuItem>
        <DropdownMenuItem disabled={draft}>
          <Clock aria-hidden="true" />
          <DropdownMenuItemText>Extend deadline</DropdownMenuItemText>
        </DropdownMenuItem>
        <DropdownMenuItem>
          <Copy aria-hidden="true" />
          <DropdownMenuItemText>Duplicate</DropdownMenuItemText>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" disabled={draft}>
          <CircleStop aria-hidden="true" />
          <DropdownMenuItemText>Close early</DropdownMenuItemText>
        </DropdownMenuItem>
      </RowMenu>
    </div>
  );
}

export const cases: Record<string, () => ReactElement> = {
  "row-menu": () => assignmentMenu(false),

  "row-menu-disabled": () => assignmentMenu(true),

  "row-menu-bottom": () => (
    <div className="flex min-h-[calc(100svh-3rem)] items-end justify-end">
      {assignmentMenu(false)}
    </div>
  ),

  "row-menu-middle": () => (
    <div className="flex min-h-[calc(100svh-3rem)] items-center justify-end">
      {assignmentMenu(false)}
    </div>
  ),

  "check-menu": function ClassFilter() {
    const [chosen, setChosen] = useState<readonly string[]>([]);
    return (
      <div className="flex justify-end">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm">
              Class
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="data-[scale=deck]:w-60">
            <DropdownMenuLabel>Filter by class</DropdownMenuLabel>
            {CLASSES.map((name) => (
              <DropdownMenuCheckboxItem
                key={name}
                checked={chosen.includes(name)}
                onCheckedChange={() => setChosen(toggled(chosen, name))}
              >
                <DropdownMenuItemText>{name}</DropdownMenuItemText>
              </DropdownMenuCheckboxItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              disabled={chosen.length === 0}
              onSelect={() => setChosen([])}
            >
              <X aria-hidden="true" />
              <DropdownMenuItemText>Clear filter</DropdownMenuItemText>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    );
  },

  "meta-menu": () => (
    <div className="flex justify-end">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm">
            <CircleDot aria-hidden="true" />
            Single choice
            <ChevronDown aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="data-[scale=deck]:w-62.5">
          <DropdownMenuLabel>Question type</DropdownMenuLabel>
          {QUESTION_TYPES.map(({ label, icon: Icon }, index) => (
            <DropdownMenuItem key={label}>
              <Icon aria-hidden="true" />
              <DropdownMenuItemText>{label}</DropdownMenuItemText>
              {index === 0 && <DropdownMenuMeta>Current</DropdownMenuMeta>}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  ),

  "long-labels": function LongLabels() {
    const [status, setStatus] = useState("live");
    const [chosen, setChosen] = useState<readonly string[]>([]);
    const first = "Lớp luyện thi IELTS 6.5 buổi tối thứ Hai, thứ Tư và thứ Sáu";
    const second = "Lớp TOEIC 600 cuối tuần";
    return (
      <div className="flex flex-col gap-6">
        <div className="flex items-center justify-between gap-3">
          <Segmented
            label="Trạng thái"
            scroll
            value={status}
            onChange={setStatus}
            options={LONG_STATUSES}
          />
          <RowMenu title="Bài đã giao: Đề thi thử giữa kỳ kỹ năng Đọc hiểu, đợt tháng Mười">
            <DropdownMenuItem>
              <ArrowUpRight aria-hidden="true" />
              <DropdownMenuItemText>
                Mở trang theo dõi bài làm của cả lớp
              </DropdownMenuItemText>
              <DropdownMenuMeta>24</DropdownMenuMeta>
            </DropdownMenuItem>
            <DropdownMenuItem>
              <Clock aria-hidden="true" />
              Gia hạn thời gian nộp bài cho những học viên chưa nộp
            </DropdownMenuItem>
            <DropdownMenuCheckboxItem
              checked={chosen.includes(first)}
              onCheckedChange={() => setChosen(toggled(chosen, first))}
            >
              <DropdownMenuItemText>{first}</DropdownMenuItemText>
            </DropdownMenuCheckboxItem>
            <DropdownMenuCheckboxItem
              checked={chosen.includes(second)}
              onCheckedChange={() => setChosen(toggled(chosen, second))}
            >
              <DropdownMenuItemText>{second}</DropdownMenuItemText>
            </DropdownMenuCheckboxItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive">
              <CircleStop aria-hidden="true" />
              <DropdownMenuItemText>Đóng bài sớm</DropdownMenuItemText>
            </DropdownMenuItem>
            <DropdownMenuItem variant="destructive" disabled>
              <X aria-hidden="true" />
              <DropdownMenuItemText>Xoá bài đã giao</DropdownMenuItemText>
            </DropdownMenuItem>
          </RowMenu>
        </div>
        <Tabs defaultValue="students" className="flex flex-col gap-4">
          <TabsList aria-label="Bài đã giao">
            <TabsTrigger value="students">Học viên và tiến độ làm bài</TabsTrigger>
            <TabsTrigger value="questions">Câu hỏi và phân tích từng câu</TabsTrigger>
            <TabsTrigger value="settings">Cài đặt bài đã giao</TabsTrigger>
          </TabsList>
          <TabsContent value="students">Học viên</TabsContent>
          <TabsContent value="questions">Câu hỏi</TabsContent>
          <TabsContent value="settings">Cài đặt</TabsContent>
        </Tabs>
        <div className="text-ui flex items-center gap-2">
          <Checkbox id="long-labels-notify" defaultChecked />
          <label htmlFor="long-labels-notify">
            Thông báo cho học viên qua email khi có thay đổi
          </label>
        </div>
      </div>
    );
  },

  "segmented-counts": function AssignmentTabs() {
    const [tab, setTab] = useState("live");
    return (
      <Segmented
        label="Status"
        scroll
        value={tab}
        onChange={setTab}
        options={[
          { value: "live", label: "Live", count: 3 },
          { value: "scheduled", label: "Scheduled", count: 1 },
          { value: "closed", label: "Closed", count: 1 },
          { value: "draft", label: "Drafts", count: 1 },
        ]}
      />
    );
  },

  "segmented-scroll": function ImportFilters() {
    const [filter, setFilter] = useState("all");
    return (
      <Segmented
        label="Status"
        scroll
        value={filter}
        onChange={setFilter}
        options={[
          { value: "all", label: "All", count: 9 },
          { value: "processing", label: "Processing", count: 1 },
          { value: "review", label: "Ready for review", count: 4 },
          { value: "failed", label: "Couldn’t process", count: 2 },
          { value: "draft", label: "Draft created", count: 1 },
          { value: "cancelled", label: "Cancelled", count: 1 },
        ]}
      />
    );
  },

  "segmented-sizes": function Sizes() {
    const [show, setShow] = useState("all");
    const [mode, setMode] = useState("rich");
    const [source, setSource] = useState("file");
    return (
      <div className="flex flex-col items-start gap-4">
        <Segmented
          label="Show"
          size="sm"
          scroll
          value={show}
          onChange={setShow}
          options={[
            { value: "all", label: "All" },
            { value: "action", label: "Needs action" },
            { value: "confirm", label: "To confirm" },
            { value: "info", label: "Info" },
          ]}
        />
        <Segmented
          label="Prompt editing mode"
          size="xs"
          value={mode}
          onChange={setMode}
          options={[
            { value: "rich", label: "Rich text", icon: Type },
            { value: "md", label: "Markdown", icon: FileCode },
          ]}
        />
        <Segmented
          label="Source"
          size="lg"
          scroll
          value={source}
          onChange={setSource}
          options={[
            { value: "file", label: "Upload a file", icon: FileUp },
            { value: "paste", label: "Paste text", icon: ClipboardPaste },
          ]}
        />
      </div>
    );
  },

  checkbox: function Checkboxes() {
    const [all, setAll] = useState(true);
    const [row, setRow] = useState(false);
    return (
      <div className="flex items-center gap-4">
        <Checkbox
          aria-label="Select row"
          checked={row}
          onChange={(event) => setRow(event.target.checked)}
        />
        <Checkbox
          aria-label="Select all on page"
          checked={all}
          onChange={(event) => setAll(event.target.checked)}
        />
        <Checkbox
          aria-label="Some selected"
          aria-checked="mixed"
          checked={false}
          onChange={() => {}}
          ref={(element) => {
            if (element) element.indeterminate = true;
          }}
        />
        <Checkbox aria-label="Disabled" disabled checked={false} onChange={() => {}} />
      </div>
    );
  },

  tabs: () => (
    <Tabs defaultValue="students" className="flex flex-col gap-4">
      <TabsList aria-label="Assignment">
        <TabsTrigger value="students">Students</TabsTrigger>
        <TabsTrigger value="questions">Questions</TabsTrigger>
        <TabsTrigger value="settings">Settings</TabsTrigger>
      </TabsList>
      <TabsContent value="students">Students</TabsContent>
      <TabsContent value="questions">Questions</TabsContent>
      <TabsContent value="settings">Settings</TabsContent>
    </Tabs>
  ),

  "student-account-menu": function StudentAccountMenu() {
    const [client] = useState(() => {
      useAuthStore.setState({ user: STUDENT });
      return new QueryClient();
    });
    return (
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <div className="flex justify-end">
            <AccountMenu settingsTo="/app/settings" deck />
          </div>
        </MemoryRouter>
      </QueryClientProvider>
    );
  },
};
