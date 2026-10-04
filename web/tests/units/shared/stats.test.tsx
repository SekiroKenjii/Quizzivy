import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { Activity, Clock, Flag, SquarePen } from "lucide-react";
import { KpiTile, type KpiTileProps } from "@/components/shared/stats/KpiTile";
import { Meter } from "@/components/shared/stats/Meter";
import { ProgressBar } from "@/components/shared/stats/ProgressBar";
import { StatStrip } from "@/components/shared/stats/StatStrip";
import { percent, shares, thresholdTone } from "@/components/shared/stats/progress";
import { contentWidth } from "@tests/support/contentWidth";
import { viewport } from "@tests/support/viewport";

afterEach(() => {
  vi.unstubAllGlobals();
});

const TO_GRADE = {
  label: "Cần chấm",
  icon: SquarePen,
  tone: "warning",
  value: "6",
  hint: "4 học viên · lâu nhất 2 ngày",
} as const;

const NOTHING_FLAGGED = {
  label: "Bài làm bị gắn cờ",
  icon: Flag,
  tone: "danger",
  value: "0",
  hint: "Không có bài nào bị gắn cờ",
} as const;

type Fits<T> = T extends KpiTileProps ? "accepted" : "refused";

function tile(props: KpiTileProps) {
  return render(
    <MemoryRouter>
      <KpiTile {...props} />
    </MemoryRouter>,
  );
}

describe("KpiTile with a destination", () => {
  it("is one link to its route, named by its label, value, hint and action", () => {
    tile({ ...TO_GRADE, to: "/teacher/grading", action: "Chấm" });
    const link = screen.getByRole("link", {
      name: "Cần chấm 6 4 học viên · lâu nhất 2 ngày Chấm",
    });
    expect(link).toHaveAttribute("href", "/teacher/grading");
    expect(screen.getAllByRole("link")).toHaveLength(1);
    expect(screen.queryByRole("button")).toBeNull();
    expect(link.querySelector("svg.lucide-arrow-right")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  });

  it("darkens its border under the pointer", () => {
    tile({ ...TO_GRADE, to: "/teacher/grading", action: "Chấm" });
    expect(screen.getByRole("link")).toHaveClass("hover:border-ring");
  });

  it("leads the label with the live dot only when it is live", () => {
    const quiet = tile({ ...TO_GRADE, to: "/teacher/grading", action: "Chấm" });
    expect(quiet.container.querySelector(".qz-live-dot")).toBeNull();
    quiet.unmount();

    const live = tile({
      label: "Đang làm bài",
      icon: Activity,
      tone: "success",
      value: "5",
      hint: "Ở 2 bài đã giao",
      live: true,
      to: "/teacher/assignments",
      action: "Theo dõi",
    });
    const dot = live.container.querySelector(".qz-live-dot");
    expect(dot).not.toBeNull();
    expect(dot!.nextElementSibling).toHaveTextContent("Đang làm bài");
    expect(
      screen.getByRole("link", { name: "Đang làm bài 5 Ở 2 bài đã giao Theo dõi" }),
    ).toBeInTheDocument();
  });

  it.each([
    ["warning", SquarePen, "lucide-square-pen", "text-warning"],
    ["danger", Flag, "lucide-flag", "text-danger"],
    ["muted", Clock, "lucide-clock", "text-muted-fg"],
    ["success", Activity, "lucide-activity", "text-success"],
  ] as const)("draws the %s icon in its solid tone", (tone, icon, glyph, ink) => {
    const { container } = tile({
      ...TO_GRADE,
      tone,
      icon,
      to: "/teacher/grading",
      action: "Chấm",
    });
    const drawn = container.querySelector(`svg.${glyph}`);
    expect(drawn).toHaveClass(ink, "size-4");
    expect(drawn).toHaveAttribute("aria-hidden", "true");
    for (const other of [
      "text-warning",
      "text-danger",
      "text-muted-fg",
      "text-success",
    ])
      if (other !== ink) expect(drawn).not.toHaveClass(other);
  });

  it("gives each row of the deck's button a line height of its own", () => {
    const { container } = tile({ ...TO_GRADE, to: "/teacher/grading", action: "Chấm" });
    const [head, value, foot] = [...container.querySelector("a")!.children];
    expect(head).toHaveClass("text-sm", "leading-4");
    expect(value).toHaveClass("text-kpi", "leading-[1.15]", "tabular-nums");
    expect(foot).toHaveClass("text-meta", "leading-4");
  });
});

describe("KpiTile without a destination", () => {
  it("is a plain card: no link, no button, no action and no tab stop", async () => {
    const user = userEvent.setup();
    const { container } = tile(NOTHING_FLAGGED);
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByText("Bài làm bị gắn cờ")).toBeInTheDocument();
    expect(screen.getByText("0")).toBeInTheDocument();
    expect(screen.getByText("Không có bài nào bị gắn cờ")).toBeInTheDocument();
    expect(container.querySelector("svg.lucide-flag")).toHaveClass("text-danger");
    expect(container.querySelector("svg.lucide-arrow-right")).toBeNull();
    expect(container.querySelector("a, button, [href], [tabindex], [role]")).toBeNull();
    expect(container.firstElementChild).not.toHaveClass("hover:border-ring");
    expect(container.firstElementChild!.lastElementChild!.children).toHaveLength(1);
    await user.tab();
    expect(document.body).toHaveFocus();
  });

  it("keeps the frame of the tile that links", () => {
    const linked = tile({ ...TO_GRADE, to: "/teacher/grading", action: "Chấm" });
    const frame = [...linked.container.firstElementChild!.classList].filter(
      (name) => name !== "hover:border-ring" && name !== "transition-colors",
    );
    linked.unmount();
    const plain = tile(NOTHING_FLAGGED);
    expect([...plain.container.firstElementChild!.classList]).toEqual(frame);
    expect(frame).toEqual(
      expect.arrayContaining(["bg-card", "shadow-card", "rounded-xl", "border"]),
    );
  });

  it("has types that refuse a route without its action and an action without a route", () => {
    type Figure = typeof NOTHING_FLAGGED;
    const verdicts: [
      Fits<Figure>,
      Fits<Figure & { to: string; action: string }>,
      Fits<Figure & { to: string }>,
      Fits<Figure & { action: string }>,
    ] = ["accepted", "accepted", "refused", "refused"];
    expect(verdicts).toHaveLength(4);
  });
});

const STATS = [
  { label: "Đã nộp", value: "18", suffix: " / 24" },
  { label: "Đang làm", value: "2" },
  { label: "Điểm trung bình", value: "78", suffix: "%" },
  { label: "Bị gắn cờ", value: "1", suffix: " / 24", tone: "danger" },
] as const;

function strip() {
  const { container } = render(<StatStrip items={STATS} />);
  return container.querySelector("dl")!;
}

describe("StatStrip", () => {
  it("is a description list of four terms and four definitions", () => {
    strip();
    expect(screen.getAllByRole("term").map((term) => term.textContent)).toEqual([
      "Đã nộp",
      "Đang làm",
      "Điểm trung bình",
      "Bị gắn cờ",
    ]);
    expect(screen.getAllByRole("definition").map((value) => value.textContent)).toEqual(
      ["18 / 24", "2", "78%", "1 / 24"],
    );
  });

  it("is four columns while no content area is registered", () => {
    const list = strip();
    expect(list).toHaveClass("grid-cols-4");
    expect(list).not.toHaveClass("grid-cols-2");
  });

  it("is four columns at a content width of exactly 640", () => {
    contentWidth(640);
    const wide = strip();
    expect(wide).toHaveClass("grid-cols-4");
    expect(wide).not.toHaveClass("grid-cols-2");
  });

  it("is two columns one pixel below 640", () => {
    contentWidth(639);
    const narrow = strip();
    expect(narrow).toHaveClass("grid-cols-2");
    expect(narrow).not.toHaveClass("grid-cols-4");
  });

  it("changes when the content area crosses 640, in both directions", () => {
    const area = contentWidth(976);
    const list = strip();
    expect(list).toHaveClass("grid-cols-4");
    area.resize(332);
    expect(list).toHaveClass("grid-cols-2");
    area.resize(640);
    expect(list).toHaveClass("grid-cols-4");
  });

  it("follows the content area, not the viewport", () => {
    viewport("desktop");
    contentWidth(332);
    const narrow = strip();
    expect(narrow).toHaveClass("grid-cols-2");
  });

  it("stays four columns on a phone viewport when the content area is wide", () => {
    viewport("phone");
    contentWidth(976);
    expect(strip()).toHaveClass("grid-cols-4");
  });

  it("puts the danger tone on the value and leaves the suffix muted", () => {
    strip();
    const [submitted, , , flagged] = screen.getAllByRole("definition");
    expect(submitted).toHaveClass("text-fg", "text-stat", "leading-normal");
    expect(submitted).not.toHaveClass("text-danger-ink");
    expect(flagged).toHaveClass("text-danger-ink");
    expect(flagged).not.toHaveClass("text-fg");
    const suffix = flagged!.querySelector("span");
    expect(suffix).toHaveTextContent("/ 24");
    expect(suffix).toHaveClass("text-muted-fg", "text-sm");
    expect(suffix).not.toHaveClass("text-danger-ink");
  });

  it("draws no suffix for a value that has none", () => {
    strip();
    expect(screen.getAllByRole("definition")[1]!.querySelector("span")).toBeNull();
  });
});

function bar(name: string) {
  const track = screen.getByRole("progressbar", { name });
  return { track, fill: track.firstElementChild as HTMLElement };
}

describe("ProgressBar", () => {
  it("is a progressbar whose value is the whole percentage, 75 for 18 of 24", () => {
    render(<ProgressBar value={18} max={24} label="Đã nộp 18/24" />);
    const { track, fill } = bar("Đã nộp 18/24");
    expect(track).toHaveAttribute("aria-valuemin", "0");
    expect(track).toHaveAttribute("aria-valuemax", "100");
    expect(track).toHaveAttribute("aria-valuenow", "75");
    expect(fill.style.width).toBe("75%");
    expect(track).toHaveClass("bg-muted", "overflow-hidden");
  });

  it("takes a percentage when no max is given", () => {
    render(<ProgressBar value={44} label="Tỉ lệ đúng" />);
    const { track, fill } = bar("Tỉ lệ đúng");
    expect(track).toHaveAttribute("aria-valuenow", "44");
    expect(fill.style.width).toBe("44%");
  });

  it("is empty when nobody is assigned", () => {
    render(<ProgressBar value={0} max={0} label="Đã nộp 0/0" />);
    const { track, fill } = bar("Đã nộp 0/0");
    expect(track).toHaveAttribute("aria-valuenow", "0");
    expect(fill.style.width).toBe("0%");
  });

  it("stops at the ends of the track", () => {
    render(
      <>
        <ProgressBar value={30} max={24} label="Quá số" />
        <ProgressBar value={-5} max={24} label="Dưới không" />
      </>,
    );
    expect(bar("Quá số").track).toHaveAttribute("aria-valuenow", "100");
    expect(bar("Quá số").fill.style.width).toBe("100%");
    expect(bar("Dưới không").track).toHaveAttribute("aria-valuenow", "0");
    expect(bar("Dưới không").fill.style.width).toBe("0%");
  });

  it.each([
    ["accent", "bg-brand"],
    ["danger", "bg-danger"],
    ["warning", "bg-warning"],
    ["success", "bg-success"],
  ] as const)("fills in the %s tone", (tone, fillClass) => {
    render(<ProgressBar value={50} label="Tỉ lệ đúng" tone={tone} />);
    const { fill } = bar("Tỉ lệ đúng");
    expect(fill).toHaveClass(fillClass);
    for (const other of ["bg-brand", "bg-danger", "bg-warning", "bg-success"])
      if (other !== fillClass) expect(fill).not.toHaveClass(other);
  });

  it("is the accent colour, 6px and square-ended unless told otherwise", () => {
    render(<ProgressBar value={50} label="Tỉ lệ đúng" />);
    const { track, fill } = bar("Tỉ lệ đúng");
    expect(track).toHaveClass("h-1.5", "rounded-[0.375rem]");
    expect(fill).toHaveClass("bg-brand");
    expect(fill.className).not.toContain("rounded");
  });

  it("rounds the end of the fill with the round cap", () => {
    render(<ProgressBar value={50} label="Tỉ lệ đúng" cap="round" />);
    expect(bar("Tỉ lệ đúng").fill).toHaveClass("rounded-[0.375rem]");
  });

  it("is 8px on an 8px radius at size 8", () => {
    render(<ProgressBar value={50} label="Kỹ năng đọc" size={8} cap="round" />);
    const { track, fill } = bar("Kỹ năng đọc");
    expect(track).toHaveClass("h-2", "rounded-[0.5rem]");
    expect(track).not.toHaveClass("h-1.5");
    expect(fill).toHaveClass("rounded-[0.5rem]");
  });

  it("takes its width from the caller", () => {
    render(<ProgressBar value={50} label="Tỉ lệ đúng" className="flex-1" />);
    expect(bar("Tỉ lệ đúng").track).toHaveClass("flex-1", "block");
  });
});

describe("percent", () => {
  it("is the share of max in whole percent", () => {
    expect(percent(18, 24)).toBe(75);
    expect(percent(44)).toBe(44);
    expect(percent(1, 3)).toBe(33);
    expect(percent(2, 3)).toBe(67);
  });

  it("rounds a half up, as Math.round does", () => {
    expect(percent(1, 8)).toBe(13);
    expect(percent(5, 8)).toBe(63);
    expect(percent(1, 200)).toBe(1);
  });

  it("is 0 when max is zero or less, without dividing", () => {
    expect(percent(0, 0)).toBe(0);
    expect(percent(5, 0)).toBe(0);
    expect(percent(5, -10)).toBe(0);
    expect(percent(Number.NaN, 10)).toBe(0);
  });

  it("stays between 0 and 100", () => {
    expect(percent(30, 24)).toBe(100);
    expect(percent(-5, 24)).toBe(0);
    expect(percent(Number.POSITIVE_INFINITY, 24)).toBe(100);
  });
});

describe("thresholdTone", () => {
  it.each([
    [0, "danger"],
    [22, "danger"],
    [39, "danger"],
    [40, "warning"],
    [44, "warning"],
    [64, "warning"],
    [65, "success"],
    [83, "success"],
    [100, "success"],
  ] as const)("%i percent is %s", (value, tone) => {
    expect(thresholdTone(value)).toBe(tone);
  });
});

const STORAGE = [
  { key: "audio", value: 1.05, tone: "accent" },
  { key: "images", value: 0.2, tone: "info" },
] as const;

const STORAGE_TEXT = "Đã dùng 1,25 GB trên 5 GB: audio 1,05 GB, ảnh 0,2 GB";

function parts(meter: HTMLElement) {
  return [...meter.children] as HTMLElement[];
}

describe("Meter", () => {
  it("is a meter named by its label whose value is the sum of its parts", () => {
    render(
      <Meter label="Dung lượng" valueText={STORAGE_TEXT} max={5} parts={STORAGE} />,
    );
    const meter = screen.getByRole("meter", { name: "Dung lượng" });
    expect(meter).toHaveAttribute("aria-valuemin", "0");
    expect(meter).toHaveAttribute("aria-valuemax", "5");
    expect(meter).toHaveAttribute("aria-valuenow", "1.25");
    expect(meter).toHaveAttribute("aria-valuetext", STORAGE_TEXT);
  });

  it("draws 1.05 and 0.2 of 5 as 21% and 4%, each in its tone", () => {
    render(
      <Meter label="Dung lượng" valueText={STORAGE_TEXT} max={5} parts={STORAGE} />,
    );
    const [audio, images] = parts(screen.getByRole("meter"));
    expect(audio!.style.width).toBe("21%");
    expect(audio).toHaveClass("bg-brand");
    expect(images!.style.width).toBe("4%");
    expect(images).toHaveClass("bg-info");
    expect(screen.getByRole("meter")).toHaveClass("h-2", "rounded-[0.5rem]", "flex");
  });

  it("stops at the end of the track when the parts add up to more than max", () => {
    render(
      <Meter
        label="Dung lượng"
        valueText="Đã dùng 7 GB trên 5 GB"
        max={5}
        parts={[
          { key: "audio", value: 4, tone: "warning" },
          { key: "images", value: 3, tone: "danger" },
        ]}
      />,
    );
    const meter = screen.getByRole("meter");
    expect(meter).toHaveAttribute("aria-valuenow", "5");
    const [audio, images] = parts(meter);
    expect(audio!.style.width).toBe("80%");
    expect(audio).toHaveClass("bg-warning");
    expect(images!.style.width).toBe("20%");
    expect(images).toHaveClass("bg-danger");
  });

  it("draws nothing for an empty store", () => {
    render(<Meter label="Dung lượng" valueText="Chưa dùng" max={5} parts={[]} />);
    const meter = screen.getByRole("meter");
    expect(meter).toHaveAttribute("aria-valuenow", "0");
    expect(meter).toBeEmptyDOMElement();
  });

  it("does not count a negative part in its value", () => {
    render(
      <Meter
        label="Dung lượng"
        valueText="Đã dùng 2 GB trên 4 GB"
        max={4}
        parts={[
          { key: "audio", value: -1, tone: "accent" },
          { key: "images", value: 2, tone: "info" },
        ]}
      />,
    );
    const meter = screen.getByRole("meter");
    expect(meter).toHaveAttribute("aria-valuenow", "2");
    expect(parts(meter).map((part) => part.style.width)).toEqual(["0%", "50%"]);
  });

  it("leaves a part that is not a number out of its value, as out of its fill", () => {
    render(
      <Meter
        label="Dung lượng"
        valueText={STORAGE_TEXT}
        max={5}
        parts={[
          { key: "audio", value: Number.NaN, tone: "accent" },
          { key: "images", value: 2, tone: "info" },
        ]}
      />,
    );
    const meter = screen.getByRole("meter");
    expect(meter).toHaveAttribute("aria-valuenow", "2");
    expect(parts(meter).map((part) => part.style.width)).toEqual(["0%", "40%"]);
  });
});

describe("shares", () => {
  it("is each value's share of max, not rounded to a whole percent", () => {
    expect(shares([1.05, 0.2], 5)).toEqual([21, 4]);
    expect(shares([1, 1, 1], 3)).toEqual([33.33, 33.33, 33.33]);
    expect(shares([0.5], 16)).toEqual([3.13]);
  });

  it("cuts the part that runs past the end and gives nothing to those after it", () => {
    expect(shares([4, 3], 5)).toEqual([80, 20]);
    expect(shares([6, 1], 5)).toEqual([100, 0]);
    expect(shares([2, 2, 2, 2], 5)).toEqual([40, 40, 20, 0]);
  });

  it("gives nothing to a negative value or on a track of no size", () => {
    expect(shares([-1, 2], 4)).toEqual([0, 50]);
    expect(shares([1, 2], 0)).toEqual([0, 0]);
    expect(shares([Number.NaN, 2], 4)).toEqual([0, 50]);
  });
});
