import { useState, type ReactElement } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { Bell, Lock, Palette, SlidersHorizontal, User } from "lucide-react";
import { CopyField } from "@/components/shared/CopyField";
import { DirtyBar } from "@/components/shared/DirtyBar";
import {
  SettingsLayout,
  type SettingsSection,
} from "@/components/shared/SettingsLayout";
import { Sheet } from "@/components/shared/Sheet";
import { ChipInput } from "@/components/shared/form/ChipInput";
import { NumberStepper } from "@/components/shared/form/NumberStepper";
import { RadioCards } from "@/components/shared/form/RadioCard";
import { TagCombobox } from "@/components/shared/form/TagCombobox";
import type { TagSuggestion } from "@/components/shared/form/tagOptions";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";

const EVENTS = [
  ["Started attempt", "19:22"],
  ["Left fullscreen · 14 s", "19:41"],
  ["Switched tab · 32 s", "19:58"],
  ["Left fullscreen · 8 s", "20:06"],
  ["Submitted", "20:14"],
];

const FILTERS = [
  { label: "Type", options: ["Single choice", "Multiple choice", "Fill in the blank"] },
  { label: "Level", options: ["A1", "A2", "B1", "B2"] },
  { label: "Skill", options: ["Reading", "Listening", "Grammar"] },
];

const TAG_COUNTS: readonly (readonly [string, number])[] = [
  ["grammar", 2],
  ["reading", 2],
  ["listening", 1],
  ["map", 1],
  ["part 2", 1],
  ["prepositions", 1],
  ["present perfect", 1],
  ["speaking", 1],
  ["task 1", 1],
  ["TFNG", 1],
  ["travel", 1],
  ["vocabulary", 1],
  ["writing", 1],
  ["A2", 0],
  ["B1", 0],
  ["B2", 0],
  ["conditionals", 0],
  ["IELTS", 0],
  ["passive voice", 0],
  ["past simple", 0],
  ["pronunciation", 0],
  ["reported speech", 0],
  ["TOEIC", 0],
  ["unit 4", 0],
  ["unit 5", 0],
];

function tagMeta(count: number) {
  if (count === 0) return "Suggested";
  return count === 1 ? "1 question" : `${count} questions`;
}

const TAGS: readonly TagSuggestion[] = TAG_COUNTS.map(([tag, count]) => ({
  tag,
  meta: tagMeta(count),
}));

function card(title: string, text: string): ReactElement {
  return (
    <div className="bg-card shadow-card rounded-xl border p-4.5">
      <h2 className="text-md font-semibold">{title}</h2>
      <p className="text-muted-fg text-sm">{text}</p>
    </div>
  );
}

const SECTIONS: readonly SettingsSection[] = [
  {
    id: "profile",
    label: "Profile",
    icon: User,
    to: "/settings",
    content: (
      <>
        {card("Profile", "Your name, email and time zone.")}
        {card("About you", "Shown to your students beside your name.")}
      </>
    ),
  },
  {
    id: "security",
    label: "Sign-in & security",
    icon: Lock,
    to: "/settings/security",
    content: card("Sign-in & security", "Password, Google and devices."),
  },
  {
    id: "notif",
    label: "Notifications",
    icon: Bell,
    to: "/settings/notif",
    content: card("Notifications", "What reaches you, and how often."),
  },
  {
    id: "grading",
    label: "Assignment defaults",
    icon: SlidersHorizontal,
    to: "/settings/grading",
    content: card("Assignment defaults", "What a new assignment starts with."),
  },
  {
    id: "appearance",
    label: "Appearance",
    icon: Palette,
    to: "/settings/appearance",
    content: card("Appearance", "Theme and table density."),
  },
];

function opened(label: string, onOpen: () => void, sheet: ReactElement): ReactElement {
  return (
    <main tabIndex={-1} className="min-h-40">
      <Button variant="outline" onClick={onOpen}>
        {label}
      </Button>
      {sheet}
    </main>
  );
}

const unsaved = (error: string | null) =>
  function Unsaved() {
    const [dirty, setDirty] = useState(true);
    return (
      <form
        className="flex flex-col gap-3.5"
        onSubmit={(event) => {
          event.preventDefault();
          setDirty(false);
        }}
      >
        <div className="bg-card shadow-card flex flex-col gap-1.5 rounded-xl border p-4.5">
          <label htmlFor="harness-full-name" className="text-meta font-medium">
            Full name
          </label>
          <input
            id="harness-full-name"
            className="border-input bg-bg text-ui h-9.5 rounded-md border px-3"
            defaultValue="Hoàng Thương"
            onChange={() => setDirty(true)}
          />
        </div>
        <DirtyBar dirty={dirty} error={error} onDiscard={() => setDirty(false)} />
      </form>
    );
  };

export const cases: Record<string, () => ReactElement> = {
  "sheet-420": function AttemptSheet() {
    const [open, setOpen] = useState(true);
    return opened(
      "Lê Hoàng Nam",
      () => setOpen(true),
      <Sheet
        open={open}
        onOpenChange={setOpen}
        title="Lê Hoàng Nam"
        subtitle="Mid-term Reading Mock"
        leading={<Avatar name="Lê Hoàng Nam" size="lg" />}
        footer={
          <>
            <Button variant="outline" className="h-9.5 flex-1 shadow-none">
              Open full attempt
            </Button>
            <Button className="h-9.5 flex-1">Grade answers</Button>
          </>
        }
      >
        <div className="grid grid-cols-2 gap-2.5">
          <div className="rounded-lg border p-3">
            <div className="text-muted-fg text-xs">Score</div>
            <div className="text-xl font-semibold tabular-nums">31 / 40</div>
          </div>
          <div className="rounded-lg border p-3">
            <div className="text-muted-fg text-xs">Time spent</div>
            <div className="text-xl font-semibold tabular-nums">52 min</div>
          </div>
        </div>
        <div>
          <div className="mb-2 text-sm font-semibold">Attempt timeline</div>
          {EVENTS.map(([text, time]) => (
            <div key={time} className="flex gap-3 pb-3.5 text-sm">
              <span className="bg-ring mt-1.25 size-2.5 flex-none rounded-full" />
              <span className="flex-1">
                <span className="block">{text}</span>
                <span className="text-muted-fg block text-xs">{time}</span>
              </span>
            </div>
          ))}
        </div>
      </Sheet>,
    );
  },

  "sheet-380": function VersionSheet() {
    const [open, setOpen] = useState(true);
    return opened(
      "Version history",
      () => setOpen(true),
      <Sheet open={open} onOpenChange={setOpen} title="Version history" width={380}>
        <div className="flex flex-col gap-1">
          {[3, 2, 1].map((version) => (
            <div
              key={version}
              className="flex flex-col gap-1.5 rounded-lg border p-2 text-xs"
            >
              <span className="text-ui font-semibold">Version {version}</span>
              <span className="text-muted-fg">2 d ago · Hoàng Thương</span>
            </div>
          ))}
        </div>
      </Sheet>,
    );
  },

  "sheet-320": function FilterSheet() {
    const [open, setOpen] = useState(true);
    return opened(
      "Filters",
      () => setOpen(true),
      <Sheet open={open} onOpenChange={setOpen} title="Filters" width={320}>
        {FILTERS.map((filter) => (
          <div key={filter.label}>
            <div className="text-meta mb-1.5 font-semibold">{filter.label}</div>
            {filter.options.map((option) => (
              <div key={option} className="px-0.5 py-1.25 text-sm">
                {option}
              </div>
            ))}
          </div>
        ))}
      </Sheet>,
    );
  },

  "sheet-long": function LongSheet() {
    const [open, setOpen] = useState(true);
    return opened(
      "Bài làm của Nguyễn Hoàng Bảo Ngọc",
      () => setOpen(true),
      <Sheet
        open={open}
        onOpenChange={setOpen}
        title="Nguyễn Hoàng Bảo Ngọc Trâm Anh"
        subtitle="Đề thi thử giữa kỳ kỹ năng Đọc hiểu, đợt tháng Mười, lớp luyện thi buổi tối"
        leading={<Avatar name="Nguyễn Hoàng Bảo Ngọc Trâm Anh" size="lg" />}
        footer={
          <>
            <Button variant="outline" className="h-9.5 flex-1 shadow-none">
              Mở toàn bộ bài làm
            </Button>
            <Button className="h-9.5 flex-1">Chấm câu trả lời</Button>
          </>
        }
      >
        {Array.from({ length: 24 }, (_, index) => (
          <p key={index} className="text-sm">
            Dòng thời gian của bài làm, sự kiện thứ {index + 1}.
          </p>
        ))}
      </Sheet>,
    );
  },

  "dirty-bar": unsaved(null),

  "dirty-bar-error": unsaved(
    "Could not save your changes. Check your connection and try again.",
  ),

  "settings-layout": () => (
    <MemoryRouter initialEntries={["/settings"]}>
      <Routes>
        {SECTIONS.map((section) => (
          <Route
            key={section.id}
            path={section.to}
            element={
              <SettingsLayout
                label="Settings"
                active={section.id}
                sections={SECTIONS}
              />
            }
          />
        ))}
      </Routes>
    </MemoryRouter>
  ),

  "number-stepper": function Steppers() {
    const [attempts, setAttempts] = useState(1);
    const [minutes, setMinutes] = useState(45);
    const [times, setTimes] = useState(2);
    const [away, setAway] = useState(3);
    return (
      <div className="flex flex-col items-start gap-4">
        <NumberStepper
          label="Attempts per student"
          value={attempts}
          min={1}
          max={99}
          onChange={setAttempts}
        />
        <NumberStepper
          label="Time limit"
          value={minutes}
          min={1}
          max={600}
          step={5}
          format={(value) => `${value} min`}
          onChange={setMinutes}
        />
        <NumberStepper
          label="Times allowed"
          value={times}
          min={1}
          max={99}
          format={(value) => `${value}×`}
          onChange={setTimes}
        />
        <NumberStepper
          label="Ignore short absences"
          value={away}
          min={0}
          max={30}
          format={(value) => `${value} s`}
          onChange={setAway}
        />
        <NumberStepper
          label="Switched off"
          value={10}
          min={0}
          max={30}
          disabled
          format={(value) => `${value} s`}
          onChange={() => {}}
        />
      </div>
    );
  },

  "radio-cards": function Modes() {
    const [mode, setMode] = useState("auto");
    return (
      <RadioCards
        label="How to read the content"
        value={mode}
        onChange={setMode}
        options={[
          {
            value: "auto",
            label: "Recognise automatically",
            hint: "Works with most free-form exams. No template needed.",
          },
          {
            value: "profile",
            label: "Use a saved profile",
            hint: "For files in a format you have imported before.",
          },
        ]}
      />
    );
  },

  "chip-input": function Answers() {
    const [gap, setGap] = useState<readonly string[]>([
      "green space",
      "public green space",
    ]);
    const [blank, setBlank] = useState<readonly string[]>(["parks", "green space"]);
    return (
      <div className="flex flex-col gap-6">
        <div
          data-variant="builder"
          className="bg-bg rounded-ctl flex items-center gap-2.5 border px-2 py-1.5"
        >
          <span className="border-ring bg-muted text-muted-fg inline-flex h-6 min-w-11 flex-none items-center justify-center rounded-sm border-[1.5px] border-dashed px-2 text-xs font-semibold">
            1
          </span>
          <ChipInput
            label="Accepted answer for gap 1"
            values={gap}
            onChange={setGap}
            placeholder="Add another accepted answer"
            removeLabel={(value) => `Remove answer ${value}`}
            tone="success"
            size="md"
            frame={false}
          />
        </div>
        <div data-variant="bank">
          <ChipInput
            label="Accepted answers for blank 1"
            values={blank}
            onChange={setBlank}
            placeholder="Accepted answer, then Enter"
            removeLabel={(value) => `Remove answer ${value}`}
            tone="success"
          />
        </div>
      </div>
    );
  },

  "tag-combobox": function Tags() {
    const [tags, setTags] = useState<readonly string[]>(["grammar", "present perfect"]);
    return (
      <div className="min-h-80">
        <TagCombobox
          label="Add tag"
          tags={tags}
          onChange={setTags}
          suggestions={TAGS}
        />
      </div>
    );
  },

  "copy-field": () => (
    <CopyField
      label="Join link"
      value="https://quizzivy.app/join/K7M2-QX9P"
      failedMessage="Could not copy. The link is selected: press Ctrl+C to copy it."
    />
  ),

  "long-labels": function LongLabels() {
    const [minutes, setMinutes] = useState(600);
    const [mode, setMode] = useState("auto");
    const [answers, setAnswers] = useState<readonly string[]>([
      "không gian xanh công cộng dành cho mọi người dân trong thành phố",
      "công viên",
    ]);
    const [tags, setTags] = useState<readonly string[]>([
      "ngữ pháp thì hiện tại hoàn thành",
      "đọc hiểu",
    ]);
    return (
      <div className="flex flex-col gap-6">
        <NumberStepper
          label="Thời gian làm bài"
          value={minutes}
          min={1}
          max={600}
          step={5}
          format={(value) => `${value} phút`}
          onChange={setMinutes}
        />
        <RadioCards
          label="Cách đọc nội dung"
          value={mode}
          onChange={setMode}
          options={[
            {
              value: "auto",
              label: "Tự nhận dạng cấu trúc của đề thi",
              hint: "Phù hợp với hầu hết đề thi soạn tự do. Bạn không cần chuẩn bị mẫu nào.",
            },
            {
              value: "profile",
              label: "Dùng hồ sơ nhận dạng đã lưu",
              hint: "Dành cho tệp có định dạng mà bạn đã nhập trước đây.",
            },
            { value: "manual", label: "Tự đánh dấu từng câu hỏi", disabled: true },
          ]}
        />
        <ChipInput
          label="Đáp án chấp nhận"
          values={answers}
          onChange={setAnswers}
          placeholder="Nhập đáp án rồi nhấn Enter"
          removeLabel={(value) => `Bỏ đáp án ${value}`}
          tone="success"
          invalid={answers.length === 0}
        />
        <div className="min-h-72">
          <TagCombobox
            label="Thêm thẻ"
            tags={tags}
            onChange={setTags}
            suggestions={[
              { tag: "ngữ pháp câu điều kiện loại hai và loại ba", meta: "12 câu hỏi" },
              { tag: "nghe hiểu", meta: "3 câu hỏi" },
              { tag: "Nghé con" },
            ]}
          />
        </div>
        <CopyField
          label="Liên kết tham gia lớp luyện thi IELTS 6.5 buổi tối"
          value="https://quizzivy.app/join/K7M2-QX9P?lop=ielts-6-5-buoi-toi-thu-hai-thu-tu-thu-sau"
          copyLabel="Sao chép liên kết"
          failedMessage="Không sao chép được. Liên kết đã được chọn: hãy nhấn Ctrl+C để sao chép."
        />
        <DirtyBar
          dirty
          error="Không lưu được thay đổi vì mất kết nối. Hãy kiểm tra mạng rồi thử lại."
          onDiscard={() => {}}
          onSave={() => {}}
        />
      </div>
    );
  },
};
