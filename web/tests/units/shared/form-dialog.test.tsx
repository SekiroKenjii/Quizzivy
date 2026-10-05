import { useState } from "react";
import { it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, within, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeckScale } from "@/components/ui/deck-scale";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { TagCombobox } from "@/components/shared/form/TagCombobox";
import { NumberStepper } from "@/components/shared/form/NumberStepper";
import "@/lib/i18n";
type Values = {
  title: string;
  hidden: string;
  flag: boolean;
  choices: readonly string[];
};
const initial: Values = { title: "", hidden: "", flag: false, choices: [] };
const fields: readonly FormField<Values>[] = [
  { kind: "text", name: "title", label: "Tên", required: true, hint: "Tên mới" },
  { kind: "text", name: "hidden", label: "Ẩn", required: true, when: (v) => v.flag },
  { kind: "toggle", name: "flag", text: "Hiện" },
];
const base = {
  open: true,
  onOpenChange: () => {},
  title: "Biểu mẫu",
  initial,
  fields,
  submitLabel: "Lưu",
};
it("keeps invalid submit pressable, reveals required errors on press and focuses the first control", async () => {
  const submit = vi.fn();
  render(<FormDialog {...base} onSubmit={submit} />);
  const button = screen.getByRole("button", { name: "Lưu" });
  expect(button).toHaveAttribute("aria-disabled", "true");
  expect(button).not.toBeDisabled();
  expect(screen.queryByText("Bắt buộc")).toBeNull();
  await userEvent.setup().click(button);
  expect(submit).not.toHaveBeenCalled();
  const input = screen.getByRole("textbox", { name: "Tên" });
  expect(input).toHaveFocus();
  expect(input).toHaveAttribute("aria-invalid", "true");
  expect(input).toHaveAccessibleDescription("Tên mới Bắt buộc");
  expect(screen.getByText("Bắt buộc")).toBeVisible();
});
it("submits valid typed values once through the public form event without closing", async () => {
  const user = userEvent.setup();
  const submit = vi.fn();
  const close = vi.fn();
  render(<FormDialog {...base} onSubmit={submit} onOpenChange={close} />);
  await user.type(screen.getByRole("textbox", { name: "Tên" }), "Bài mới");
  const form = document.querySelector("form")!;
  const input = screen.getByRole<HTMLInputElement>("textbox", { name: "Tên" });
  const button = screen.getByRole<HTMLButtonElement>("button", { name: "Lưu" });
  expect(input.form).toBe(form);
  expect(button.form).toBe(form);
  expect(form).not.toContainElement(button);
  expect(form.elements).toContain(button);
  fireEvent.submit(form);
  expect(submit).toHaveBeenCalledExactlyOnceWith({ ...initial, title: "Bài mới" });
  expect(close).not.toHaveBeenCalled();
  expect(screen.getByRole("dialog")).toBeVisible();
});
it.each(["pending", "disabled"] as const)(
  "guards %s against button, Enter and direct submit",
  async (flag) => {
    const submit = vi.fn();
    render(
      <FormDialog
        {...base}
        initial={{ ...initial, title: "Có tên" }}
        {...{ [flag]: true }}
        onSubmit={submit}
      />,
    );
    expect(screen.getByRole("button", { name: "Lưu" })).toBeDisabled();
    fireEvent.submit(document.querySelector("form")!);
    expect(submit).not.toHaveBeenCalled();
    fireEvent.keyDown(screen.getByRole("textbox", { name: "Tên" }), { key: "Enter" });
    expect(submit).not.toHaveBeenCalled();
  },
);
it("retains hidden values and ignores caller errors for hidden fields", async () => {
  const user = userEvent.setup();
  const submit = vi.fn();
  render(
    <FormDialog
      {...base}
      initial={{ ...initial, title: "Đủ", hidden: "Giữ" }}
      validate={() => ({ hidden: "Sai" })}
      onSubmit={submit}
    />,
  );
  expect(screen.queryByRole("textbox", { name: "Ẩn" })).toBeNull();
  await user.click(screen.getByRole("button", { name: "Lưu" }));
  expect(submit).toHaveBeenCalledExactlyOnceWith({
    ...initial,
    title: "Đủ",
    hidden: "Giữ",
  });
  await user.click(screen.getByRole("switch", { name: "Hiện" }));
  expect(screen.getByRole("textbox", { name: "Ẩn" })).toHaveValue("Giữ");
});
it("shows caller validation on the field after submission and keeps values with the alert", async () => {
  render(
    <FormDialog
      {...base}
      initial={{ ...initial, title: "Có tên" }}
      validate={() => ({ title: "Không dùng tên này" })}
      error="Lưu thất bại"
      onSubmit={() => {}}
    />,
  );
  expect(screen.queryByText("Không dùng tên này")).toBeNull();
  await userEvent.setup().click(screen.getByRole("button", { name: "Lưu" }));
  expect(screen.getByRole("textbox", { name: "Tên" })).toHaveValue("Có tên");
  expect(screen.getByRole("textbox", { name: "Tên" })).toHaveFocus();
  expect(screen.getByRole("alert")).toHaveTextContent("Lưu thất bại");
  expect(screen.getByText("Không dùng tên này")).toBeVisible();
});
it("fresh equivalent props keep typing, while closing and reopening resets values and touched", async () => {
  const submit = vi.fn();
  const view = render(<FormDialog {...base} onSubmit={submit} />);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Lưu" }));
  await user.type(screen.getByRole("textbox", { name: "Tên" }), "Bản nháp");
  view.rerender(
    <FormDialog
      {...base}
      initial={{ ...initial }}
      fields={[...fields]}
      onSubmit={submit}
    />,
  );
  expect(screen.getByRole("textbox", { name: "Tên" })).toHaveValue("Bản nháp");
  view.rerender(<FormDialog {...base} open={false} onSubmit={submit} />);
  view.rerender(<FormDialog {...base} onSubmit={submit} />);
  expect(screen.getByRole("textbox", { name: "Tên" })).toHaveValue("");
  expect(screen.queryByText("Bắt buộc")).toBeNull();
});
it("derives title, description and submit label from the draft", async () => {
  render(
    <FormDialog
      {...base}
      title={(v) => `Sửa ${v.title}`}
      description={(v) => `Mô tả ${v.title}`}
      submitLabel={(v) => `Lưu ${v.title}`}
      onSubmit={() => {}}
    />,
  );
  await userEvent.setup().type(screen.getByRole("textbox", { name: "Tên" }), "An");
  expect(screen.getByRole("dialog", { name: "Sửa An" })).toHaveTextContent("Mô tả An");
  expect(screen.getByRole("button", { name: "Lưu An" })).toBeVisible();
});
it("danger confirmation retains primitive variant and legacy child Enter cannot confirm", async () => {
  const user = userEvent.setup();
  const confirm = vi.fn();
  render(
    <ConfirmDialog
      open
      title="Xóa?"
      confirmLabel="Xóa"
      destructive
      onOpenChange={() => {}}
      onConfirm={confirm}
    >
      <input aria-label="Ghi chú" />
    </ConfirmDialog>,
  );
  await user.type(screen.getByRole("textbox", { name: "Ghi chú" }), "Ghi{Enter}");
  expect(confirm).not.toHaveBeenCalled();
  const button = screen.getByRole("button", { name: "Xóa" });
  expect(button).toHaveAttribute("data-variant", "destructive");
  expect(document.querySelector(".lucide-triangle-alert")).toBeInTheDocument();
  await user.click(button);
  expect(confirm).toHaveBeenCalledTimes(1);
});
it("plain confirm and notice keep their respective variant and single footer action", async () => {
  const close = vi.fn();
  render(
    <ConfirmDialog
      open
      title="Thông báo"
      confirmLabel="Hiểu rồi"
      onOpenChange={close}
    />,
  );
  const button = screen.getByRole("button", { name: "Hiểu rồi" });
  expect(button).toHaveAttribute("data-variant", "default");
  expect(button.parentElement!.querySelectorAll("button")).toHaveLength(1);
  await userEvent.setup().click(button);
  expect(close).toHaveBeenCalledWith(false);
});
it("groups focus their first real control after invalid submission", async () => {
  render(
    <FormDialog
      {...base}
      fields={[
        {
          kind: "chips",
          name: "choices",
          label: "Lớp",
          required: true,
          options: [{ value: "a", label: "Lớp A" }],
        },
      ]}
      onSubmit={() => {}}
    />,
  );
  await userEvent.setup().click(screen.getByRole("button", { name: "Lưu" }));
  expect(screen.getByRole("button", { name: "Lớp A" })).toHaveFocus();
  expect(
    screen.getByRole("group", { name: "Lớp" }).closest("[data-field-name]"),
  ).toHaveAttribute("aria-invalid", "true");
});
function Nested({ kind }: Readonly<{ kind: "tag" | "dirty" | "empty" | "unchanged" }>) {
  const [open, setOpen] = useState(true);
  const [number, setNumber] = useState(45);
  const [tags, setTags] = useState<readonly string[]>([]);
  return (
    <DeckScale>
      <FormDialog
        {...base}
        open={open}
        onOpenChange={setOpen}
        initial={{ ...initial, title: "Giữ tên" }}
        onSubmit={() => {}}
      >
        {kind === "tag" ? (
          <TagCombobox
            label="Thẻ"
            tags={tags}
            onChange={setTags}
            suggestions={[{ tag: "Reading" }]}
          />
        ) : (
          <NumberStepper
            label="Giới hạn"
            value={number}
            onChange={setNumber}
            min={0}
            max={100}
          />
        )}
      </FormDialog>
    </DeckScale>
  );
}
it.each(["tag", "dirty", "empty"] as const)(
  "first native Escape restores %s locally and second closes the FormDialog",
  async (kind) => {
    const user = userEvent.setup();
    render(<Nested kind={kind} />);
    const control = screen.getByRole(kind === "tag" ? "combobox" : "spinbutton", {
      name: kind === "tag" ? "Thẻ" : "Giới hạn",
    });
    await user.click(control);
    if (kind !== "tag") {
      await user.clear(control);
      if (kind === "dirty") await user.type(control, "67");
    } else await user.type(control, "Re");
    await user.keyboard("{Escape}");
    expect(screen.getByRole("dialog", { name: "Biểu mẫu" })).toBeVisible();
    expect(control).toHaveFocus();
    expect(screen.getByRole("textbox", { name: "Tên" })).toHaveValue("Giữ tên");
    if (kind === "tag") expect(control).toHaveAttribute("aria-expanded", "false");
    else expect(control).toHaveValue("45");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
  },
);
it("unchanged number and ordinary field dismiss on the first Escape", async () => {
  const user = userEvent.setup();
  render(<Nested kind="unchanged" />);
  await user.click(screen.getByRole("spinbutton", { name: "Giới hạn" }));
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
});

beforeEach(() =>
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  ),
);
afterEach(() => vi.unstubAllGlobals());

it.each(["date", "time"] as const)(
  "focuses and describes the invalid %s trigger in either scale",
  async (kind) => {
    const user = userEvent.setup();
    const node = (
      <FormDialog
        open
        onOpenChange={() => {}}
        title="Ngày giờ"
        initial={{ value: "" }}
        fields={[
          { kind, name: "value", label: "Mốc", hint: "Giờ địa phương", required: true },
        ]}
        submitLabel="Lưu"
        onSubmit={() => {}}
      />
    );
    const view = render(<DeckScale>{node}</DeckScale>);
    await user.click(screen.getByRole("button", { name: "Lưu" }));
    const trigger = screen.getByRole("button", { name: "Mốc" });
    expect(trigger).toHaveFocus();
    expect(trigger).toHaveAttribute("aria-invalid", "true");
    expect(trigger).toHaveAccessibleDescription("Giờ địa phương Bắt buộc");
    view.unmount();
    render(node);
    await user.click(screen.getByRole("button", { name: "Lưu" }));
    const legacyTrigger = screen.getByRole("button", {
      name: /Mốc/,
    });
    expect(legacyTrigger).toHaveFocus();
    expect(legacyTrigger).toHaveAttribute("aria-invalid", "true");
    expect(legacyTrigger).toHaveAccessibleDescription("Giờ địa phương Bắt buộc");
  },
);
it("disables native segmented and picker buttons while pending", () => {
  render(
    <DeckScale>
      <FormDialog
        open
        pending
        onOpenChange={() => {}}
        title="Đang lưu"
        initial={{ one: "a", day: "2026-09-24" }}
        fields={[
          {
            kind: "seg",
            name: "one",
            label: "Loại",
            options: [{ value: "a", label: "Một" }],
          },
          { kind: "date", name: "day", label: "Ngày" },
        ]}
        submitLabel="Lưu"
        onSubmit={() => {}}
      />
    </DeckScale>,
  );
  expect(screen.getByRole("button", { name: "Một" })).toBeDisabled();
  expect(screen.getByRole("button", { name: /Ngày/ })).toBeDisabled();
});

const PORTAL_SCENARIOS = {
  date: {
    controlRole: "button",
    portalRole: "dialog",
    portalName: "Chọn ngày",
    initial: "2026-09-24",
    next: "2026-09-25",
  },
  time: {
    controlRole: "button",
    portalRole: "dialog",
    portalName: "Chọn giờ",
    initial: "08:07",
    next: "08:10",
  },
  select: {
    controlRole: "combobox",
    portalRole: "listbox",
    portalName: null,
    initial: "a",
    next: "b",
  },
} as const;
async function choosePortalValue(
  user: ReturnType<typeof userEvent.setup>,
  kind: keyof typeof PORTAL_SCENARIOS,
  portal: HTMLElement,
) {
  const controls = within(portal);
  switch (kind) {
    case "date":
      await user.click(controls.getByRole("button", { name: /ngày 25 tháng 09/ }));
      break;
    case "time":
      await user.click(controls.getByRole("button", { name: "Tăng phút" }));
      await user.click(controls.getByRole("button", { name: "Xong" }));
      break;
    case "select":
      await user.click(controls.getByRole("option", { name: "Lớp B" }));
      break;
  }
}
it.each([
  { kind: "date", lock: "pending" },
  { kind: "date", lock: "disabled" },
  { kind: "time", lock: "pending" },
  { kind: "time", lock: "disabled" },
  { kind: "select", lock: "pending" },
  { kind: "select", lock: "disabled" },
] as const)(
  "discards the open $kind portal on $lock without changing saved or other form drafts",
  async ({ kind, lock }) => {
    const user = userEvent.setup();
    const submit = vi.fn();
    const scenario = PORTAL_SCENARIOS[kind];
    const initial = { title: "", value: scenario.initial };
    const field: FormField<typeof initial> =
      kind === "select"
        ? {
            kind,
            name: "value",
            label: "Mốc",
            options: [
              { value: "a", label: "Lớp A" },
              { value: "b", label: "Lớp B" },
            ],
          }
        : { kind, name: "value", label: "Mốc" };
    const props = {
      open: true,
      onOpenChange: () => {},
      title: "Đổi trạng thái",
      initial,
      fields: [
        { kind: "text", name: "title", label: "Tên" },
        field,
      ] as readonly FormField<typeof initial>[],
      submitLabel: "Lưu",
      onSubmit: submit,
    };
    const renderForm = (locked: boolean) => (
      <DeckScale>
        <FormDialog {...props} {...{ [lock]: locked }} />
      </DeckScale>
    );
    const portalOptions = scenario.portalName ? { name: scenario.portalName } : {};
    const view = render(renderForm(false));
    await user.type(screen.getByRole("textbox", { name: "Tên" }), "Giữ bản nháp");
    await user.click(screen.getByRole(scenario.controlRole, { name: /Mốc/ }));
    if (kind === "time")
      await user.click(screen.getByRole("button", { name: "Tăng phút" }));
    const portal = screen.getByRole(scenario.portalRole, portalOptions);
    view.rerender(renderForm(true));
    expect.soft(screen.queryByRole(scenario.portalRole, portalOptions)).toBeNull();
    if (portal.isConnected) await choosePortalValue(user, kind, portal);
    expect(submit).not.toHaveBeenCalled();
    view.rerender(renderForm(false));
    expect(screen.queryByRole(scenario.portalRole, portalOptions)).toBeNull();
    expect(screen.getByRole("textbox", { name: "Tên" })).toHaveValue("Giữ bản nháp");
    await user.click(screen.getByRole("button", { name: "Lưu" }));
    expect
      .soft(submit)
      .toHaveBeenCalledExactlyOnceWith({ ...initial, title: "Giữ bản nháp" });
    submit.mockClear();
    await user.click(screen.getByRole(scenario.controlRole, { name: /Mốc/ }));
    await choosePortalValue(
      user,
      kind,
      screen.getByRole(scenario.portalRole, portalOptions),
    );
    await user.click(screen.getByRole("button", { name: "Lưu" }));
    expect(submit).toHaveBeenCalledExactlyOnceWith({
      title: "Giữ bản nháp",
      value: scenario.next,
    });
  },
);
it.each(["pending", "disabled", "closed"] as const)(
  "rejects a retained native file callback delivered while %s",
  async (lock) => {
    const add = vi.spyOn(window, "addEventListener");
    const submit = vi.fn();
    const validate = vi.fn((_values: { title: string; file: File | null }) => null);
    const initial = { title: "", file: null as File | null };
    const props = {
      open: true,
      onOpenChange: () => {},
      title: "Tệp",
      initial,
      fields: [
        { kind: "text", name: "title", label: "Tên" },
        { kind: "file", name: "file", label: "Tệp" },
      ] as readonly FormField<typeof initial>[],
      validate,
      submitLabel: "Lưu",
      onSubmit: submit,
    };
    const renderForm = (locked: boolean) => (
      <FormDialog
        {...props}
        open={lock === "closed" ? !locked : true}
        pending={lock === "pending" && locked}
        disabled={lock === "disabled" && locked}
      />
    );
    const view = render(renderForm(false));
    const listener = add.mock.calls.find(([name]) => name === "drop")?.[1];
    add.mockRestore();
    expect(listener).toBeTypeOf("function");
    await userEvent.setup().type(screen.getByRole("textbox", { name: "Tên" }), "Giữ");
    view.rerender(renderForm(true));
    validate.mockClear();
    const event = new Event("drop", { cancelable: true });
    Object.defineProperty(event, "dataTransfer", {
      value: { types: ["Files"], files: [new File(["x"], "late.mp3")] },
    });
    act(() => {
      if (typeof listener === "function") listener(event);
    });
    expect(validate.mock.calls.every(([values]) => values.file === null)).toBe(true);
    view.rerender(renderForm(false));
    await userEvent.setup().click(screen.getByRole("button", { name: "Lưu" }));
    expect(submit).toHaveBeenCalledExactlyOnceWith({
      title: lock === "closed" ? "" : "Giữ",
      file: null,
    });
  },
);

it("rejects a callback retained by a removed file field after the form locks", async () => {
  const user = userEvent.setup();
  const add = vi.spyOn(window, "addEventListener");
  const initial = { title: "", show: true, file: null as File | null };
  const validate = vi.fn((_values: typeof initial) => null);
  const submit = vi.fn();
  const props = {
    open: true,
    onOpenChange: () => {},
    title: "Tệp",
    initial,
    fields: [
      { kind: "text", name: "title", label: "Tên" },
      { kind: "toggle", name: "show", text: "Hiện tệp" },
      { kind: "file", name: "file", label: "Tệp", when: (values) => values.show },
    ] as readonly FormField<typeof initial>[],
    validate,
    submitLabel: "Lưu",
    onSubmit: submit,
  };
  const view = render(<FormDialog {...props} />);
  const listener = add.mock.calls.find(([name]) => name === "drop")?.[1];
  add.mockRestore();
  expect(listener).toBeTypeOf("function");
  await user.type(screen.getByRole("textbox", { name: "Tên" }), "Giữ");
  await user.click(screen.getByRole("switch", { name: "Hiện tệp" }));
  expect(document.querySelector('input[type="file"]')).toBeNull();
  view.rerender(<FormDialog {...props} pending />);
  validate.mockClear();
  const event = new Event("drop", { cancelable: true });
  Object.defineProperty(event, "dataTransfer", {
    value: { types: ["Files"], files: [new File(["x"], "retired.mp3")] },
  });
  act(() => {
    if (typeof listener === "function") listener(event);
  });
  expect(validate.mock.calls.every(([values]) => values.file === null)).toBe(true);
  view.rerender(<FormDialog {...props} />);
  await user.click(screen.getByRole("button", { name: "Lưu" }));
  expect(submit).toHaveBeenCalledExactlyOnceWith({
    title: "Giữ",
    show: false,
    file: null,
  });
});
