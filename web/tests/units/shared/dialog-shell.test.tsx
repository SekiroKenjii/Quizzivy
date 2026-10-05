import { useRef, useState, useEffect } from "react";
import { it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeckScale } from "@/components/ui/deck-scale";
import {
  DialogShell,
  DialogShellHeader,
  DialogShellBody,
  DialogShellFooter,
} from "@/components/shared/form/DialogShell";
import "@/lib/i18n";
const shell = (width = 480) => (
  <DialogShell open width={width} onOpenChange={() => {}}>
    <DialogShellHeader title="Tên" />
    <DialogShellBody>Nội dung</DialogShellBody>
    <DialogShellFooter>
      <button>Tiếp</button>
    </DialogShellFooter>
  </DialogShell>
);
it.each([480, 520, 540, 560, 680, 720])(
  "uses the exact %i width and an instant frame",
  (width) => {
    render(shell(width));
    const content = screen.getByRole("dialog", { name: "Tên" });
    expect(content.style.getPropertyValue("--dialog-width")).toBe(`${width}px`);
    expect(content).toHaveClass(
      "max-h-[86dvh]",
      "data-[state=open]:animate-none",
      "data-[state=closed]:animate-none",
    );
    expect(document.querySelector('[data-slot="dialog-overlay"]')).toHaveClass(
      "data-[state=open]:animate-none",
      "data-[state=closed]:animate-none",
    );
  },
);
it("has the same frame inside and outside deck context apart from its z class", () => {
  render(shell());
  const before = screen.getByRole("dialog").className;
  cleanup();
  render(<DeckScale>{shell()}</DeckScale>);
  expect(
    screen
      .getByRole("dialog")
      .className.split(" ")
      .filter((c) => !c.includes("scale=deck"))
      .sort((a, b) => a.localeCompare(b)),
  ).toEqual(before.split(" ").sort((a, b) => a.localeCompare(b)));
});
it.each(["escape", "overlay", "close"])("dismisses through %s", async (method) => {
  const change = vi.fn();
  render(
    <DialogShell open onOpenChange={change}>
      <DialogShellHeader title="Tên" />
    </DialogShell>,
  );
  if (method === "escape")
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
  if (method === "overlay")
    await userEvent
      .setup()
      .click(document.querySelector('[data-slot="dialog-overlay"]')!);
  if (method === "close")
    await userEvent.setup().click(screen.getByRole("button", { name: "Đóng" }));
  await waitFor(() => expect(change).toHaveBeenCalledWith(false));
});
function FocusCase({
  explicit = false,
  remove = false,
  moved = false,
  detached = false,
}: Readonly<{
  explicit?: boolean;
  remove?: boolean;
  moved?: boolean;
  detached?: boolean;
}>) {
  const [open, setOpen] = useState(false);
  const [gone, setGone] = useState(false);
  const target = useRef<HTMLInputElement>(null);
  const [finished, setFinished] = useState(false);
  useEffect(() => {
    if (finished && moved) target.current?.focus();
  }, [finished, moved]);
  return (
    <main tabIndex={-1} data-testid="page">
      {!gone && <button onClick={() => setOpen(true)}>Mở</button>}
      <input aria-label="Ngoài" ref={target} />
      <DialogShell
        open={open}
        onOpenChange={setOpen}
        {...(explicit
          ? {
              returnFocus: detached
                ? { current: document.createElement("button") }
                : target,
            }
          : {})}
      >
        <DialogShellHeader title="Tên" />
        <button
          onClick={() => {
            if (remove) setGone(true);
            setOpen(false);
            setFinished(true);
          }}
        >
          Hoàn tất
        </button>
      </DialogShell>
    </main>
  );
}
it.each([
  { explicit: false, remove: false, moved: false, detached: false, name: "Mở" },
  { explicit: true, remove: false, moved: false, detached: false, name: "Ngoài" },
  { explicit: true, remove: false, moved: false, detached: true, name: "Mở" },
  { explicit: false, remove: true, moved: false, detached: false, name: "page" },
  { explicit: false, remove: false, moved: true, detached: false, name: "Ngoài" },
])("returns connected focus correctly for $name", async ({ name, ...props }) => {
  const user = userEvent.setup();
  render(<FocusCase {...props} />);
  await user.click(screen.getByRole("button", { name: "Mở" }));
  await user.click(screen.getByRole("button", { name: "Hoàn tất" }));
  const role = name === "Ngoài" ? "textbox" : "button";
  await waitFor(() => {
    const target =
      name === "page" ? screen.getByTestId("page") : screen.getByRole(role, { name });
    expect(target).toHaveFocus();
  });
});
