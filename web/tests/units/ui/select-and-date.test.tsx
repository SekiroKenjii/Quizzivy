import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { DateTimeField, type DateTimeMode } from "@/components/shared/DateTimeField";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import "@/lib/i18n";

function Duration() {
  const [minutes, setMinutes] = useState("45");
  return (
    <>
      <Label htmlFor="duration">Thời lượng làm bài</Label>
      <Select value={minutes} onValueChange={setMinutes}>
        <SelectTrigger id="duration">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {[15, 30, 45, 60].map((m) => (
            <SelectItem key={m} value={String(m)}>
              {m} phút
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <output>{minutes}</output>
    </>
  );
}

function Window({
  mode,
  initial,
}: {
  readonly mode?: DateTimeMode;
  readonly initial: string;
}) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <Label htmlFor="closes">Đóng lúc</Label>
      <DateTimeField
        id="closes"
        label="Đóng lúc"
        value={value}
        onChange={setValue}
        {...(mode === undefined ? {} : { mode })}
      />
      <output>{value}</output>
    </>
  );
}

describe("the select (F-06)", () => {
  it("is our list, not the platform's: a labelled combobox whose options carry a check", async () => {
    const user = userEvent.setup();
    render(<Duration />);

    const trigger = screen.getByRole("combobox", { name: "Thời lượng làm bài" });
    expect(trigger).toHaveTextContent("45 phút");
    expect(document.querySelector("select")).toBeNull();

    await user.click(trigger);
    const list = await screen.findByRole("listbox");
    expect(within(list).getAllByRole("option")).toHaveLength(4);
    expect(within(list).getByRole("option", { name: "45 phút" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    await user.click(within(list).getByRole("option", { name: "60 phút" }));
    expect(screen.getByRole("status")).toHaveTextContent("60");
    expect(trigger).toHaveTextContent("60 phút");
  });
});

describe("the date-time field (F-06)", () => {
  it("is one named trigger, never a datetime-local or a joined pair", () => {
    render(<Window initial="2026-09-08T15:00" />);

    expect(document.querySelector("input[type=datetime-local]")).toBeNull();
    expect(document.querySelector("input[type=time]")).toBeNull();
    expect(screen.getAllByRole("button", { name: /Đóng lúc/ })).toHaveLength(1);
    const field = screen.getByRole("button", { name: /Đóng lúc/ });
    expect(field).toHaveTextContent("15:00");
    expect(field).not.toHaveClass("rounded-r-none");
  });

  it("keeps a minute that is off the step", () => {
    render(<Window initial="2026-09-08T09:32" />);

    expect(screen.getByRole("button", { name: /Đóng lúc/ })).toHaveTextContent("09:32");
    expect(screen.getByRole("status")).toHaveTextContent("2026-09-08T09:32");
  });

  it("shows only the half the caller asked for", () => {
    const { unmount } = render(<Window mode="date" initial="2026-09-08" />);
    expect(screen.getByRole("button", { name: /Đóng lúc/ })).not.toHaveTextContent(
      /\d{2}:\d{2}/,
    );
    unmount();

    render(<Window mode="time" initial="21:00" />);
    const clock = screen.getByRole("button", { name: /Đóng lúc/ });
    expect(clock).toHaveTextContent("21:00");
    expect(clock).not.toHaveTextContent("2026");
  });
});
