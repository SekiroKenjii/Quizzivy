import { useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Sheet } from "@/components/shared/Sheet";
import { DeckScale } from "@/components/ui/deck-scale";
import "@/lib/i18n";

type Width = 320 | 380 | 420;

const panel = (name = "Lê Hoàng Nam") => screen.getByRole("dialog", { name });
const slot = (name: string) => document.querySelector(`[data-slot="${name}"]`);

function open(
  props: Partial<{
    onOpenChange: (open: boolean) => void;
    subtitle: string;
    leading: ReactNode;
    width: Width;
    footer: ReactNode;
  }> = {},
) {
  return render(
    <Sheet open onOpenChange={() => {}} title="Lê Hoàng Nam" {...props}>
      <p>Dòng thời gian</p>
    </Sheet>,
  );
}

function Page() {
  const [shown, setShown] = useState(false);
  const [opener, setOpener] = useState(true);
  return (
    <main tabIndex={-1} aria-label="Bài đã giao">
      {opener && (
        <button type="button" onClick={() => setShown(true)}>
          Xem bài làm
        </button>
      )}
      <button type="button">Nút khác</button>
      <Sheet open={shown} onOpenChange={setShown} title="Lê Hoàng Nam">
        <button type="button" onClick={() => setOpener(false)}>
          Bỏ học viên khỏi danh sách
        </button>
      </Sheet>
    </main>
  );
}

describe("the sheet", () => {
  it("is a dialog named by its title, with its content", () => {
    open();
    expect(within(panel()).getByRole("heading", { name: "Lê Hoàng Nam" })).toHaveClass(
      "text-md",
      "font-semibold",
    );
    expect(within(panel()).getByText("Dòng thời gian")).toBeInTheDocument();
  });

  it("renders nothing while it is closed", () => {
    render(
      <Sheet open={false} onOpenChange={() => {}} title="Lê Hoàng Nam">
        <p>Dòng thời gian</p>
      </Sheet>,
    );
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByText("Dòng thời gian")).toBeNull();
  });

  it("draws the subtitle under the title and describes the dialog with it", () => {
    open({ subtitle: "Mid-term Reading Mock" });
    expect(panel()).toHaveAccessibleDescription("Mid-term Reading Mock");
    const subtitle = within(panel()).getByText("Mid-term Reading Mock");
    expect(subtitle).toHaveClass("text-meta", "text-muted-fg");
    expect(
      within(panel()).getByRole("heading").compareDocumentPosition(subtitle) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("has no description, and warns of none, without a subtitle", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    open();
    expect(panel()).not.toHaveAttribute("aria-describedby");
    expect(warn).not.toHaveBeenCalled();
    warn.mockRestore();
  });

  it("draws what leads the header before the title", () => {
    open({ leading: <span data-testid="avatar">NL</span> });
    const header = slot("sheet-header")!;
    expect(header.firstElementChild).toBe(screen.getByTestId("avatar"));
    expect(header.lastElementChild).toBe(
      within(panel()).getByRole("button", { name: "Đóng" }),
    );
  });

  it("renders a footer only when it is given one", () => {
    const without = open();
    expect(slot("sheet-footer")).toBeNull();
    without.unmount();

    open({ footer: <button type="button">Chấm bài</button> });
    expect(slot("sheet-footer")).toContainElement(
      screen.getByRole("button", { name: "Chấm bài" }),
    );
    expect(slot("sheet-footer")).toHaveClass("border-t", "flex-none");
  });

  it("scrolls its body and nothing else", () => {
    open({ footer: <button type="button">Chấm bài</button> });
    const body = slot("sheet-body")!;
    expect(body).toContainElement(screen.getByText("Dòng thời gian"));
    expect(body).toHaveClass("overflow-y-auto", "min-h-0", "flex-1");
    for (const other of [panel(), slot("sheet-header")!, slot("sheet-footer")!]) {
      expect(other.className).not.toMatch(/overflow/);
    }
    expect(slot("sheet-header")).toHaveClass("flex-none");
  });
});

describe("closing the sheet", () => {
  it("asks to close on Escape", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    open({ onOpenChange });
    await user.keyboard("{Escape}");
    expect(onOpenChange).toHaveBeenCalledTimes(1);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("asks to close on a press of the backdrop", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    open({ onOpenChange });
    await user.click(slot("sheet-overlay")!);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("asks to close from its close button", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    open({ onOpenChange });
    await user.click(within(panel()).getByRole("button", { name: "Đóng" }));
    expect(onOpenChange).toHaveBeenCalledTimes(1);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("stays open for a press inside it", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    open({ onOpenChange });
    await user.click(screen.getByText("Dòng thời gian"));
    expect(onOpenChange).not.toHaveBeenCalled();
  });
});

describe("the sheet's focus", () => {
  it("moves inside when it opens and back to the opener when it closes", async () => {
    const user = userEvent.setup();
    render(<Page />);
    const opener = screen.getByRole("button", { name: "Xem bài làm" });
    await user.click(opener);
    expect(panel()).toContainElement(document.activeElement as HTMLElement);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(opener).toHaveFocus();
  });

  it("is modal: the page behind is hidden from assistive technology and Tab stays inside", async () => {
    const user = userEvent.setup();
    render(<Page />);
    await user.click(screen.getByRole("button", { name: "Xem bài làm" }));
    const main = screen.getByRole("main", { hidden: true });
    expect(main.closest('[aria-hidden="true"]')).not.toBeNull();
    expect(panel().closest('[aria-hidden="true"]')).toBeNull();
    for (let press = 0; press < 5; press++) {
      await user.tab();
      expect(panel()).toContainElement(document.activeElement as HTMLElement);
    }

    await user.keyboard("{Escape}");
    expect(screen.getByRole("main").closest('[aria-hidden="true"]')).toBeNull();
  });

  it("returns to the opener from the close button too", async () => {
    const user = userEvent.setup();
    render(<Page />);
    const opener = screen.getByRole("button", { name: "Xem bài làm" });
    await user.click(opener);
    await user.click(screen.getByRole("button", { name: "Đóng" }));
    expect(opener).toHaveFocus();
  });

  it("goes to the page when the opener was removed while it was open", async () => {
    const user = userEvent.setup();
    render(<Page />);
    await user.click(screen.getByRole("button", { name: "Xem bài làm" }));
    await user.click(
      screen.getByRole("button", { name: "Bỏ học viên khỏi danh sách" }),
    );
    expect(screen.queryByRole("button", { name: "Xem bài làm" })).toBeNull();

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("main")).toHaveFocus();
  });
});

describe("the sheet's frame", () => {
  it.each([
    [320, "w-[min(320px,100%)]"],
    [380, "w-[min(380px,100%)]"],
    [420, "w-[min(420px,100%)]"],
  ] as const)("is %ipx wide, or the whole width of a narrower window", (width, cls) => {
    open({ width });
    expect(panel()).toHaveClass(cls);
    expect(
      panel()
        .className.split(" ")
        .filter((c) => c.startsWith("w-")),
    ).toEqual([cls]);
  });

  it("is 420px wide unless it is told otherwise", () => {
    open();
    expect(panel()).toHaveClass("w-[min(420px,100%)]");
  });

  it("is pinned to the right edge at the full height, on the card surface", () => {
    open();
    expect(panel()).toHaveClass(
      "fixed",
      "inset-y-0",
      "right-0",
      "flex",
      "flex-col",
      "bg-card",
      "border-l",
      "shadow-float",
    );
  });

  it("pads the 420px sheet 16 by 18, 18 and 14 by 18, with ruled header and footer", () => {
    open({ width: 420, footer: <span>Chân</span> });
    expect(slot("sheet-header")).toHaveClass("border-b", "px-4.5", "py-4", "gap-3");
    expect(slot("sheet-body")).toHaveClass("p-4.5", "gap-4.5", "flex-col");
    expect(slot("sheet-footer")).toHaveClass("px-4.5", "py-3.5", "gap-2");
  });

  it("pads the 380px sheet 14 by 16 and 14", () => {
    open({ width: 380 });
    expect(slot("sheet-header")).toHaveClass("border-b", "px-4", "py-3.5");
    expect(slot("sheet-body")).toHaveClass("p-3.5");
    expect(slot("sheet-body")!.className).not.toMatch(/gap-/);
  });

  it("draws the 320px sheet as one 16 by 18 column with no rule under its header", () => {
    open({ width: 320 });
    expect(slot("sheet-header")).toHaveClass("px-4.5", "pt-4");
    expect(slot("sheet-header")).not.toHaveClass("border-b");
    expect(slot("sheet-body")).toHaveClass("px-4.5", "pt-4.5", "pb-4", "gap-4.5");
  });

  it("draws the deck's 32px close button with a 17px cross", () => {
    open();
    const close = within(panel()).getByRole("button", { name: "Đóng" });
    expect(close).toHaveClass("size-8", "rounded-seg", "hover:bg-hover");
    expect(close.querySelector("svg")).toHaveClass("size-4.25");
    expect(close.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  it("sits with its backdrop at the sheet layer, under dialogs and menus", () => {
    open();
    for (const layer of [panel(), slot("sheet-overlay")!]) {
      expect(
        layer.className.split(" ").filter((c) => /^z-/.test(c)),
        layer.getAttribute("data-slot") ?? "",
      ).toEqual(["z-(--z-sheet)"]);
    }
    expect(slot("sheet-overlay")).toHaveClass("bg-overlay", "fixed", "inset-0");
    expect(
      slot("sheet-overlay")!.compareDocumentPosition(panel()) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("slides in from the right over 200ms and stays still for reduced motion", () => {
    open();
    expect(panel()).toHaveClass(
      "duration-200",
      "data-[state=open]:animate-in",
      "data-[state=open]:slide-in-from-right",
      "data-[state=closed]:animate-out",
      "data-[state=closed]:slide-out-to-right",
      "motion-reduce:animate-none!",
    );
    expect(slot("sheet-overlay")).toHaveClass(
      "data-[state=open]:fade-in-0",
      "data-[state=closed]:fade-out-0",
      "motion-reduce:animate-none!",
    );
  });
});

describe("the sheet on a deck surface", () => {
  it("carries the deck scale itself, because it is portalled out of the surface", () => {
    render(
      <DeckScale data-testid="surface">
        <Sheet open onOpenChange={() => {}} title="Lê Hoàng Nam">
          <p>Dòng thời gian</p>
        </Sheet>
      </DeckScale>,
    );
    expect(panel()).toHaveAttribute("data-scale", "deck");
    expect(screen.getByTestId("surface")).not.toContainElement(panel());
    expect(slot("sheet-overlay")).not.toHaveAttribute("data-scale");
  });

  it("carries no scale off a deck surface", () => {
    open();
    expect(panel()).not.toHaveAttribute("data-scale");
  });
});
