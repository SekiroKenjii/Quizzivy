import { useState } from "react";
import { it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FileDrop } from "@/components/shared/form/FileDrop";
import "@/lib/i18n";
function Box({
  onChange,
  disabled = false,
}: Readonly<{ onChange: (file: File | null) => void; disabled?: boolean }>) {
  const [value, setValue] = useState<File | null>(null);
  return (
    <FileDrop
      id="file"
      value={value}
      disabled={disabled}
      accept=".mp3"
      limits="MP3"
      onChange={(next) => {
        setValue(next);
        onChange(next);
      }}
    />
  );
}
function drop(type: string, files: File[]) {
  const event = new Event(type, { bubbles: true, cancelable: true });
  Object.defineProperty(event, "dataTransfer", { value: { types: ["Files"], files } });
  act(() => window.dispatchEvent(event));
  return event;
}
it("picks, removes and chooses the same file again with picker-only accept", async () => {
  const change = vi.fn();
  render(<Box onChange={change} />);
  const input = document.querySelector<HTMLInputElement>('input[type="file"]')!;
  expect(input).toHaveAttribute("accept", ".mp3");
  expect(screen.getByRole("button", { name: /Chọn tệp/ })).toHaveTextContent("MP3");
  const file = new File(["x"], "bài.mp3");
  const user = userEvent.setup();
  await user.upload(input, file);
  expect(screen.getByRole("button", { name: "Bỏ tệp bài.mp3" })).toHaveTextContent(
    "Bấm để bỏ tệp",
  );
  await user.click(screen.getByRole("button", { name: "Bỏ tệp bài.mp3" }));
  await user.upload(input, file);
  expect(change.mock.calls).toEqual([[file], [null], [file]]);
});
it("takes only the first dropped file without validation and clears drag state", () => {
  const change = vi.fn();
  render(<Box onChange={change} />);
  drop("dragover", []);
  expect(screen.getByRole("button")).toHaveClass("border-primary", "bg-muted");
  const first = new File(["x"], "first.any");
  const second = new File(["y"], "second.mp3");
  expect(drop("drop", [first, second]).defaultPrevented).toBe(true);
  expect(change).toHaveBeenCalledExactlyOnceWith(first);
  expect(change.mock.calls[0]![0]).toBe(first);
  expect(screen.getByRole("button", { name: "Bỏ tệp first.any" })).toBeVisible();
  expect(screen.getByRole("button")).not.toHaveClass("border-primary");
});
it("disabled stops picker and callbacks, and still keeps the window from opening a dropped file", () => {
  const change = vi.fn();
  render(<Box onChange={change} disabled />);
  expect(screen.getByRole("button")).toBeDisabled();
  expect(document.querySelector('input[type="file"]')).toBeDisabled();
  expect(drop("drop", [new File(["x"], "file.mp3")]).defaultPrevented).toBe(true);
  fireEvent.click(screen.getByRole("button"));
  expect(change).not.toHaveBeenCalled();
});
it("invalid state belongs to the box and labels the chosen-file removal", () => {
  render(<FileDrop value={new File(["x"], "file.mp3")} invalid onChange={() => {}} />);
  expect(
    screen.getByRole("button", { name: "Bỏ tệp file.mp3" }).parentElement,
  ).toHaveAttribute("aria-invalid", "true");
  expect(screen.getByRole("button")).toHaveClass("border-danger");
});
