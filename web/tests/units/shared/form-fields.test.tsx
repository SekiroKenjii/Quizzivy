import { beforeEach, afterEach, it, expect, vi } from "vitest";
import { render, screen, within, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QRCodeSVG } from "qrcode.react";
import { Link } from "lucide-react";
import { DeckScale } from "@/components/ui/deck-scale";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import "@/lib/i18n";
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
type Values = {
  text: string;
  email: string;
  number: string;
  password: string;
  area: string;
  date: string;
  time: string;
  select: string;
  seg: string;
  chips: readonly string[];
  list: readonly string[];
  toggle: boolean;
  file: File | null;
  info: string;
  qr: string;
};
const initial: Values = {
  text: "",
  email: "",
  number: "",
  password: "",
  area: "",
  date: "2026-09-24",
  time: "08:00",
  select: "a",
  seg: "a",
  chips: [],
  list: [],
  toggle: false,
  file: null,
  info: "",
  qr: "",
};
const options = [
  { value: "a", label: "Nguyễn Gia Bảo", meta: "Đang làm" },
  { value: "b", label: "Trần Minh Anh" },
  { value: "c", label: "Phạm Thu Hà" },
  { value: "d", label: "Lê Hoàng Nam" },
  { value: "e", label: "Võ Khánh Linh" },
  { value: "f", label: "Bùi Ngọc Mai" },
];
const frame = (fields: readonly FormField<Values>[], onSubmit = () => {}) => (
  <DeckScale>
    <FormDialog
      open
      onOpenChange={() => {}}
      title="Trường"
      initial={initial}
      fields={fields}
      submitLabel="Lưu"
      onSubmit={onSubmit}
    />
  </DeckScale>
);
it("marks only the six optional kinds and respects required and noOptional", () => {
  render(
    frame([
      { kind: "text", name: "text", label: "Tên" },
      { kind: "email", name: "email", label: "Email" },
      { kind: "date", name: "date", label: "Ngày" },
      { kind: "time", name: "time", label: "Giờ" },
      { kind: "area", name: "area", label: "Nội dung" },
      { kind: "file", name: "file", label: "Tệp" },
      { kind: "number", name: "number", label: "Số" },
      { kind: "password", name: "password", label: "Mật khẩu" },
      { kind: "select", name: "select", label: "Một", options },
      { kind: "text", name: "info", label: "Bắt buộc", required: true },
      { kind: "text", name: "qr", label: "Không ghi", noOptional: true },
    ]),
  );
  expect(screen.getAllByText("(không bắt buộc)")).toHaveLength(6);
  expect(screen.getByText("Số")).not.toHaveTextContent("không bắt buộc");
});
it("filters six-option list by folded Vietnamese label and meta, counts selections, and quotes no match", async () => {
  const user = userEvent.setup();
  render(frame([{ kind: "list", name: "list", label: "Học viên", options }]));
  const search = screen.getByRole("textbox", { name: "Tìm" });
  await user.type(search, "nguyen");
  expect(screen.getByRole("checkbox", { name: /Nguyễn Gia Bảo/ })).toBeVisible();
  expect(screen.queryByRole("checkbox", { name: "Trần Minh Anh" })).toBeNull();
  await user.click(screen.getByRole("checkbox", { name: /Nguyễn Gia Bảo/ }));
  await user.clear(search);
  await user.click(screen.getByRole("checkbox", { name: "Trần Minh Anh" }));
  expect(screen.getByText("Đã chọn 2")).toBeVisible();
  await user.type(search, "dang lam");
  expect(screen.getAllByRole("checkbox")).toHaveLength(1);
  await user.clear(search);
  await user.type(search, "không có");
  expect(screen.getByText("Không có mục nào khớp “không có”")).toBeVisible();
});
it.each([
  { n: 5, search: true },
  { n: 6, search: false },
])("hides search at $n options with search=$search", ({ n, search }) => {
  render(
    frame([
      {
        kind: "list",
        name: "list",
        label: "Học viên",
        options: options.slice(0, n),
        search,
      },
    ]),
  );
  expect(screen.queryByRole("textbox", { name: "Tìm" })).toBeNull();
});
it("toggles chips and list choices and submits their typed arrays", async () => {
  const submit = vi.fn();
  const user = userEvent.setup();
  render(
    frame(
      [
        { kind: "chips", name: "chips", label: "Lớp", options: options.slice(0, 2) },
        { kind: "list", name: "list", label: "Học viên", options: options.slice(0, 2) },
      ],
      submit,
    ),
  );
  const chip = within(screen.getByRole("group", { name: "Lớp" })).getByRole("button", {
    name: "Nguyễn Gia Bảo",
  });
  await user.click(chip);
  expect(chip).toHaveAttribute("aria-pressed", "true");
  await user.click(chip);
  expect(chip).toHaveAttribute("aria-pressed", "false");
  await user.click(screen.getByRole("checkbox", { name: "Trần Minh Anh" }));
  await user.click(screen.getByRole("button", { name: "Lưu" }));
  expect(submit).toHaveBeenCalledExactlyOnceWith({ ...initial, list: ["b"] });
});
it("selects and segments choices, with info actions remaining independent", async () => {
  const action = vi.fn();
  const submit = vi.fn();
  const user = userEvent.setup();
  render(
    frame(
      [
        { kind: "select", name: "select", label: "Chọn một", options },
        { kind: "seg", name: "seg", label: "Phạm vi", options: options.slice(0, 2) },
        {
          kind: "info",
          name: "info",
          rows: (v) => [
            {
              icon: Link,
              text: v.seg,
              action: { label: "Sao chép", onAction: action },
            },
          ],
        },
      ],
      submit,
    ),
  );
  await user.click(screen.getByRole("combobox", { name: "Chọn một" }));
  await user.click(screen.getByRole("option", { name: "Trần Minh Anh" }));
  await user.click(
    within(screen.getByRole("group", { name: "Phạm vi" })).getByRole("button", {
      name: "Trần Minh Anh",
    }),
  );
  await user.click(screen.getByRole("button", { name: "Sao chép" }));
  expect(action).toHaveBeenCalledTimes(1);
  expect(submit).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Lưu" }));
  expect(submit).toHaveBeenCalledExactlyOnceWith({ ...initial, select: "b", seg: "b" });
});
it("QR encodes payload independently of the displayed code", () => {
  const payload = "https://quizzivy.com/join/ABCDEF";
  const reference = render(
    <QRCodeSVG
      value={payload}
      size={150}
      bgColor="var(--paper)"
      fgColor="var(--paper-fg)"
    />,
  );
  const expected = reference.container.querySelector("svg")!.innerHTML;
  cleanup();
  render(
    frame([{ kind: "qr", name: "qr", payload, value: "ABCD-EF", text: "Đường dẫn" }]),
  );
  expect(screen.getByText("ABCD-EF")).toBeVisible();
  expect(screen.getByText("Đường dẫn")).toBeVisible();
  expect(document.querySelector('svg[height="150"]')!.innerHTML).toBe(expected);
});
it("reopening resets transient list search but equivalent initial props preserve it", async () => {
  const user = userEvent.setup();
  const fields: FormField<Values>[] = [
    { kind: "list", name: "list", label: "Học viên", options },
  ];
  const props = {
    open: true,
    onOpenChange: () => {},
    title: "Trường",
    initial,
    fields,
    submitLabel: "Lưu",
    onSubmit: () => {},
  };
  const view = render(<FormDialog {...props} />);
  await user.type(screen.getByRole("textbox", { name: "Tìm" }), "nguyen");
  view.rerender(
    <FormDialog {...props} initial={{ ...initial }} fields={[...fields]} />,
  );
  expect(screen.getByRole("textbox", { name: "Tìm" })).toHaveValue("nguyen");
  view.rerender(<FormDialog {...props} open={false} />);
  view.rerender(<FormDialog {...props} />);
  expect(screen.getByRole("textbox", { name: "Tìm" })).toHaveValue("");
});

it("keeps an empty-valued option selectable and submits its declared value", async () => {
  const submit = vi.fn();
  const user = userEvent.setup();
  render(
    frame(
      [
        {
          kind: "select",
          name: "select",
          label: "Chọn lớp",
          options: [
            { value: "a", label: "Lớp A" },
            { value: "", label: "Không chọn lớp" },
          ],
        },
      ],
      submit,
    ),
  );
  await user.click(screen.getByRole("combobox", { name: "Chọn lớp" }));
  await user.click(screen.getByRole("option", { name: "Không chọn lớp" }));
  expect(screen.getByRole("combobox", { name: "Chọn lớp" })).toHaveTextContent(
    "Không chọn lớp",
  );
  await user.click(screen.getByRole("button", { name: "Lưu" }));
  expect(submit).toHaveBeenCalledExactlyOnceWith({ ...initial, select: "" });
});
