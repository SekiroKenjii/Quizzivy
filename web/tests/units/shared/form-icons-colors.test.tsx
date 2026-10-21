import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Crown, Shield, Star } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";
import "@/lib/i18n";

type Values = { icon: string; color: string };

const fields: readonly FormField<Values>[] = [
  {
    kind: "icons",
    name: "icon",
    label: "Biểu tượng",
    options: [
      { value: "shield", label: "Khiên", icon: Shield },
      { value: "crown", label: "Vương miện", icon: Crown },
      { value: "star", label: "Ngôi sao", icon: Star },
    ],
  },
  {
    kind: "colors",
    name: "color",
    label: "Màu thẻ",
    options: [
      { value: "dark", label: "Đậm", swatch: "bg-primary text-primary-fg" },
      { value: "lime", label: "Chanh", swatch: "bg-brand-soft text-fg" },
      { value: "gray", label: "Xám", swatch: "bg-muted text-fg" },
    ],
  },
];

function frame(onSubmit: (values: Values) => void, extra: { pending?: boolean } = {}) {
  return (
    <FormDialog
      open
      onOpenChange={() => undefined}
      title="Sửa vai trò"
      initial={{ icon: "shield", color: "dark" }}
      fields={fields}
      submitLabel="Lưu"
      onSubmit={onSubmit}
      {...extra}
    />
  );
}

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

describe("the icons field", () => {
  it("is a radio group of the icons, the initial one chosen", () => {
    render(frame(() => undefined));
    const group = screen.getByRole("radiogroup", { name: "Biểu tượng" });
    const radios = within(group).getAllByRole("radio");
    expect(radios.map((radio) => radio.getAttribute("aria-label"))).toEqual([
      "Khiên",
      "Vương miện",
      "Ngôi sao",
    ]);
    expect(within(group).getByRole("radio", { name: "Khiên" })).toBeChecked();
    expect(within(group).getByRole("radio", { name: "Ngôi sao" })).not.toBeChecked();
  });

  it("chooses another icon on a click and sends it", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(frame(onSubmit));
    await user.click(screen.getByRole("radio", { name: "Vương miện" }));
    expect(screen.getByRole("radio", { name: "Vương miện" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Khiên" })).not.toBeChecked();
    await user.click(screen.getByRole("button", { name: "Lưu" }));
    expect(onSubmit).toHaveBeenCalledWith({ icon: "crown", color: "dark" });
  });

  it("moves with the arrow keys, as a radio group does", async () => {
    const user = userEvent.setup();
    render(frame(() => undefined));
    screen.getByRole("radio", { name: "Khiên" }).focus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("radio", { name: "Vương miện" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Vương miện" })).toHaveFocus();
  });

  it("names each icon by its title for the pointer", () => {
    render(frame(() => undefined));
    expect(screen.getByTitle("Ngôi sao")).toBeInTheDocument();
  });
});

describe("the colours field", () => {
  it("is a radio group of the swatches, the initial one chosen and ticked", () => {
    render(frame(() => undefined));
    const group = screen.getByRole("radiogroup", { name: "Màu thẻ" });
    expect(within(group).getAllByRole("radio")).toHaveLength(3);
    expect(within(group).getByRole("radio", { name: "Đậm" })).toBeChecked();
    expect(group.querySelectorAll("svg")).toHaveLength(1);
    expect(screen.getByTitle("Đậm").querySelector("svg")).not.toBeNull();
  });

  it("chooses another colour on a click and moves the tick", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(frame(onSubmit));
    await user.click(screen.getByRole("radio", { name: "Xám" }));
    expect(screen.getByTitle("Xám").querySelector("svg")).not.toBeNull();
    expect(screen.getByTitle("Đậm").querySelector("svg")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Lưu" }));
    expect(onSubmit).toHaveBeenCalledWith({ icon: "shield", color: "gray" });
  });

  it("paints each swatch with the classes it is given", () => {
    render(frame(() => undefined));
    expect(screen.getByTitle("Chanh")).toHaveClass("bg-brand-soft");
    expect(screen.getByTitle("Đậm")).toHaveClass("bg-primary", "text-primary-fg");
  });
});

describe("while the form saves", () => {
  it("leaves both choices unchangeable", () => {
    render(frame(() => undefined, { pending: true }));
    for (const radio of screen.getAllByRole("radio")) expect(radio).toBeDisabled();
  });
});
