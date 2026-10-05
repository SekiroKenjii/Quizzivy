import { useState, type ReactElement } from "react";
import {
  Clock,
  CalendarClock,
  Copy,
  Download,
  Hash,
  Link,
  RotateCcw,
  RotateCw,
  Upload,
} from "lucide-react";
import {
  FormDialog,
  type FormDialogProps,
  type FormValues,
  type FormOption,
} from "@/components/shared/form/FormDialog";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { DateTimeField } from "@/components/shared/DateTimeField";
import { Tooltip } from "@/components/shared/Tooltip";
import {
  DialogShell,
  DialogShellHeader,
  DialogShellBody,
  DialogShellFooter,
} from "@/components/shared/form/DialogShell";
import { SegField } from "@/components/shared/form/fields/SegField";
import { ListField } from "@/components/shared/form/fields/ListField";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";

const options = (values: readonly string[]): readonly FormOption[] =>
  values.map((value) => ({ value, label: value }));
const classes = options([
  "IELTS 6.5 Evening",
  "IELTS Foundation A",
  "TOEIC 600 Weekend",
  "Kids Starters B",
]);
const students: readonly FormOption[] = [
  { value: "Lê Hoàng Nam", label: "Lê Hoàng Nam", meta: "" },
  { value: "Trần Minh Anh", label: "Trần Minh Anh", meta: "" },
  { value: "Phạm Thu Hà", label: "Phạm Thu Hà", meta: "In progress" },
  { value: "Nguyễn Gia Bảo", label: "Nguyễn Gia Bảo", meta: "" },
  { value: "Võ Khánh Linh", label: "Võ Khánh Linh", meta: "" },
  { value: "Đặng Quốc Huy", label: "Đặng Quốc Huy", meta: "" },
  { value: "Bùi Ngọc Mai", label: "Bùi Ngọc Mai", meta: "In progress" },
  { value: "Hoàng Đức Minh", label: "Hoàng Đức Minh", meta: "Not started" },
  { value: "Đỗ Phương Thảo", label: "Đỗ Phương Thảo", meta: "" },
  { value: "Ngô Thanh Tùng", label: "Ngô Thanh Tùng", meta: "Not started" },
];
function formCase<V extends FormValues>(
  props: Omit<FormDialogProps<V>, "open" | "onOpenChange" | "onSubmit">,
) {
  return function Example(): ReactElement {
    const [open, setOpen] = useState(true);
    const [submitted, setSubmitted] = useState("");
    return (
      <main tabIndex={-1}>
        <Button variant="outline" onClick={() => setOpen(true)}>
          {typeof props.title === "string" ? props.title : "Open form"}
        </Button>
        <output aria-label="Submitted values" className="block text-xs">
          {submitted}
        </output>
        <FormDialog
          {...props}
          open={open}
          onOpenChange={setOpen}
          onSubmit={(values) => {
            setSubmitted(
              JSON.stringify(values, (_key, value: unknown) =>
                value instanceof File ? value.name : value,
              ),
            );
            setOpen(false);
          }}
        />
      </main>
    );
  };
}
const duplicate = (title: string) =>
  formCase({
    title: "Duplicate assignment",
    description: "Creates a draft with the same test and rules. Nothing is sent.",
    icon: Copy,
    initial: { title, cls: [] as readonly string[] },
    submitLabel: "Create draft",
    fields: [
      { kind: "text", name: "title", label: "Title", required: true },
      { kind: "chips", name: "cls", label: "Assign to", options: classes },
    ],
  });
const upload = (file: File | null) =>
  formCase({
    title: "Upload media",
    description: "Files can be used in any question.",
    icon: Upload,
    submitLabel: "Upload",
    initial: { kind: "Audio", file, limit: "Unlimited" },
    fields: [
      {
        kind: "seg",
        name: "kind",
        label: "Type",
        options: options(["Audio", "Image"]),
      },
      {
        kind: "file",
        name: "file",
        label: "File",
        required: true,
        accept: ".mp3,.wav,.png,.jpg",
        limits: "MP3, WAV, PNG, JPG · up to 50 MB",
      },
      {
        kind: "select",
        name: "limit",
        label: "Play limit",
        options: options(["Unlimited", "1 play", "2 plays", "3 plays"]),
        when: (values) => values.kind === "Audio",
      },
    ],
  });
function picker(mode: "date" | "time" | "datetime", initial: string, label: string) {
  return function Picker(): ReactElement {
    const [value, setValue] = useState(initial);
    return (
      <div className="max-w-96">
        <DateTimeField label={label} value={value} onChange={setValue} mode={mode} />
        <output aria-label="Wall-clock value" className="block text-xs">
          {value}
        </output>
      </div>
    );
  };
}

/** cases exposes the deck's teacher form and wall-clock picker states for independent browser comparison. */
export const cases: Record<string, () => ReactElement> = {
  "confirm-danger": function Danger(): ReactElement {
    const [open, setOpen] = useState(true);
    return (
      <main tabIndex={-1}>
        <Button variant="outline" onClick={() => setOpen(true)}>
          Close early
        </Button>
        <ConfirmDialog
          open={open}
          onOpenChange={setOpen}
          title="Close Mid-term Reading Mock now?"
          description="Students who have not started can no longer start. Attempts in progress are submitted as they are."
          confirmLabel="Close now"
          destructive
          onConfirm={() => setOpen(false)}
        />
      </main>
    );
  },
  "form-duplicate": duplicate("Mid-term Reading Mock (copy)"),
  "form-duplicate-touched": duplicate(""),
  "form-extend": function Extend(): ReactElement {
    const [open, setOpen] = useState(true);
    const [by, setBy] = useState("30 min");
    const [who, setWho] = useState("Chosen students");
    const [chosen, setChosen] = useState<readonly string[]>([]);
    const [notify, setNotify] = useState(true);
    return (
      <main tabIndex={-1}>
        <Button variant="outline" onClick={() => setOpen(true)}>
          Extend deadline
        </Button>
        <DialogShell open={open} onOpenChange={setOpen}>
          <DialogShellHeader
            title="Extend deadline"
            description="Mid-term Reading Mock · closes Today, 21:00"
            icon={Clock}
          />
          <DialogShellBody>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="extend-by" className="text-meta font-medium">
                Extend by
              </label>
              <SegField
                id="extend-by"
                label="Extend by"
                value={by}
                onChange={setBy}
                options={options(["15 min", "30 min", "1 hour", "1 day"])}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="extend-who" className="text-meta font-medium">
                For
              </label>
              <SegField
                id="extend-who"
                label="For"
                value={who}
                onChange={setWho}
                options={options(["Everyone", "Chosen students"])}
              />
            </div>
            {who === "Chosen students" && (
              <div className="flex flex-col gap-1.5">
                <label htmlFor="extend-students" className="text-meta font-medium">
                  Students
                </label>
                <ListField
                  id="extend-students"
                  label="Students"
                  value={chosen}
                  onChange={setChosen}
                  options={students}
                />
              </div>
            )}
            <div className="flex items-center gap-3">
              <label htmlFor="extend-notify" className="min-w-0 flex-1">
                <span className="text-ui block font-medium">Notify students</span>
                <span className="text-muted-fg block text-xs">
                  They get an in-app and email message
                </span>
              </label>
              <Tooltip label="They get an in-app and email message">
                <Switch
                  id="extend-notify"
                  aria-label="Notify students"
                  checked={notify}
                  onCheckedChange={setNotify}
                />
              </Tooltip>
            </div>
          </DialogShellBody>
          <DialogShellFooter>
            <Button
              variant="outline"
              className="bg-card h-9 px-3.5 shadow-none in-data-[scale=deck]:h-9 in-data-[scale=deck]:px-3.5"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button
              className="h-9 px-4 shadow-none in-data-[scale=deck]:h-9 in-data-[scale=deck]:px-4"
              onClick={() => setOpen(false)}
            >
              Extend
            </Button>
          </DialogShellFooter>
        </DialogShell>
      </main>
    );
  },
  "form-reopen": formCase({
    title: "Reopen for a student",
    description: "Gives one student a new attempt with its own time limit.",
    icon: RotateCcw,
    submitLabel: "Reopen",
    initial: { student: students[0]!.value, mins: "60 min", until: "2026-09-26" },
    fields: [
      { kind: "select", name: "student", label: "Student", options: students },
      {
        kind: "seg",
        name: "mins",
        label: "Time limit",
        options: options(["30 min", "45 min", "60 min"]),
      },
      { kind: "date", name: "until", label: "Open until" },
    ],
  }),
  "form-window": formCase({
    title: "Edit window",
    icon: CalendarClock,
    description: "When the test can be started",
    submitLabel: "Save changes",
    initial: {
      opens: "2026-09-24",
      closes: "2026-09-24",
      time: "21:00",
      cls: ["IELTS 6.5 Evening"] as readonly string[],
    },
    fields: [
      { kind: "date", name: "opens", label: "Opens", noOptional: true },
      { kind: "date", name: "closes", label: "Closes", noOptional: true },
      { kind: "time", name: "time", label: "Closing time", noOptional: true },
      { kind: "chips", name: "cls", label: "Assigned to", options: classes },
    ],
  }),
  "form-upload": upload(null),
  "form-upload-chosen": upload(
    new File(["harness"], "Unit 5 · Airport dialogue.mp3", { type: "audio/mpeg" }),
  ),
  "form-number": formCase({
    title: "Make a new code?",
    description:
      "The old code and link stop working right away. Students already in the class stay.",
    icon: RotateCw,
    submitLabel: "Make new code",
    initial: { exp: "30", max: "0" },
    fields: [
      {
        kind: "select",
        name: "exp",
        label: "Expires after",
        options: [
          { value: "30", label: "30 days" },
          { value: "7", label: "7 days" },
          { value: "90", label: "90 days" },
        ],
      },
      {
        kind: "number",
        name: "max",
        label: "Maximum uses",
        placeholder: "No limit",
        hint: "Leave empty for no limit. Up to 1,000.",
      },
    ],
    validate: (values) => {
      const text = values.max.trim();
      const value = Number(text);
      return !text || (Number.isInteger(value) && value >= 1 && value <= 1000)
        ? null
        : { max: "Use a whole number from 1 to 1,000" };
    },
  }),
  "form-join-code": formCase({
    title: "New join code",
    description:
      "Copy it now. After you close this, only the last 4 characters are shown.",
    submitLabel: "Done",
    hideCancel: true,
    initial: { qr: "", actions: "" },
    fields: [
      {
        kind: "qr",
        name: "qr",
        value: "K7QM-2PXA",
        text: "quizzivy.com/join/K7QM2PXA",
        payload: "https://quizzivy.com/join/K7QM2PXA",
      },
      {
        kind: "info",
        name: "actions",
        rows: [
          {
            icon: Link,
            text: "Join link",
            action: { label: "Copy link", onAction: () => {} },
          },
          {
            icon: Hash,
            text: "Code",
            action: { label: "Copy code", onAction: () => {} },
          },
          {
            icon: Download,
            text: "QR image for slides or Zalo",
            action: { label: "Download", onAction: () => {} },
          },
        ],
      },
    ],
  }),
  "date-time": picker("datetime", "2026-09-29T08:00", "Opens"),
  "date-only": picker("date", "2026-09-24", "Opens"),
  "time-only": picker("time", "21:00", "Closing time"),
};
