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
  it("is one joined pair of buttons, never a datetime-local", async () => {
    const user = userEvent.setup();
    render(<Window initial="2026-09-08T15:00" />);

    expect(document.querySelector("input[type=datetime-local]")).toBeNull();
    expect(document.querySelector("input[type=time]")).toBeNull();
    const day = screen.getByRole("button", { name: "Đóng lúc" });
    expect(day).toHaveTextContent("08/09/2026");
    expect(day).toHaveClass("rounded-r-none");
    const clock = screen.getByRole("button", { name: "Đóng lúc — giờ" });
    expect(clock).toHaveTextContent("15:00");
    expect(clock).toHaveClass("rounded-l-none");

    await user.click(day);
    const calendar = await screen.findByRole("dialog");
    expect(within(calendar).getByText("Tháng Chín 2026")).toBeInTheDocument();
    expect(
      within(calendar).getByRole("button", {
        name: "Thứ Ba, ngày 8 tháng 09 năm 2026, đã chọn",
      }),
    ).toBeInTheDocument();
    await user.click(
      within(calendar).getByRole("button", {
        name: "Thứ Hai, ngày 14 tháng 09 năm 2026",
      }),
    );
    expect(screen.getByRole("status")).toHaveTextContent("2026-09-14T15:00");
    expect(day).toHaveTextContent("14/09/2026");

    await user.click(clock);
    const hours = await screen.findByRole("listbox", { name: "Giờ" });
    expect(within(hours).getByRole("option", { name: "15" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await user.click(within(hours).getByRole("option", { name: "21" }));
    expect(screen.getByRole("status")).toHaveTextContent("2026-09-14T21:00");
    const minutes = screen.getByRole("listbox", { name: "Phút" });
    await user.click(within(minutes).getByRole("option", { name: "30" }));
    expect(screen.getByRole("status")).toHaveTextContent("2026-09-14T21:30");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(clock).toHaveTextContent("21:30");
  });

  it("keeps a minute that is off the step, and offers it", async () => {
    const user = userEvent.setup();
    render(<Window initial="2026-09-08T09:32" />);

    await user.click(screen.getByRole("button", { name: "Đóng lúc — giờ" }));
    const minutes = await screen.findByRole("listbox", { name: "Phút" });
    expect(within(minutes).getByRole("option", { name: "32" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(within(minutes).getAllByRole("option")).toHaveLength(13);
  });

  it("shows only the half the caller asked for", () => {
    const { unmount } = render(<Window mode="date" initial="2026-09-08" />);
    expect(screen.getByRole("button", { name: "Đóng lúc" })).toHaveTextContent(
      "08/09/2026",
    );
    expect(screen.queryByRole("button", { name: "Đóng lúc — giờ" })).toBeNull();
    unmount();

    render(<Window mode="time" initial="21:00" />);
    expect(screen.getByRole("button", { name: "Đóng lúc — giờ" })).toHaveTextContent(
      "21:00",
    );
    expect(screen.queryByText("08/09/2026")).toBeNull();
  });

  it("picking a day before any time fills in the morning", async () => {
    const user = userEvent.setup();
    render(<Window initial="" />);

    expect(screen.getByRole("button", { name: "Đóng lúc" })).toHaveTextContent(
      "Chọn ngày",
    );
    await user.click(screen.getByRole("button", { name: "Đóng lúc" }));
    const calendar = await screen.findByRole("dialog");
    await user.click(within(calendar).getAllByRole("button", { name: /ngày 15/ })[0]!);
    expect(screen.getByRole("status")).toHaveTextContent(/T08:00$/);
  });
});
