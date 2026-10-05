import { useState } from "react";
import { it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeckScale } from "@/components/ui/deck-scale";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
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
