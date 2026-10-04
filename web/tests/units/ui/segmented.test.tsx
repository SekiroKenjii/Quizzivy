import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ClipboardPaste, FileUp } from "lucide-react";
import { useState } from "react";
import { DeckScale } from "@/components/ui/deck-scale";
import { Segmented, type SegmentedOption } from "@/components/ui/segmented";

const INSIDE = "in-data-[scale=deck]:";

const classes = (element: Element) =>
  (element.getAttribute("class") ?? "").split(" ").filter(Boolean);
const plain = (element: Element) =>
  classes(element).filter((c) => !c.includes("scale=deck"));
const inside = (element: Element) =>
  classes(element)
    .filter((c) => c.startsWith(INSIDE))
    .map((c) => c.slice(INSIDE.length));

const R3_DEFAULT = `<div role="group" aria-label="Ngôn ngữ" class="bg-muted in-data-[scale=deck]:rounded-ctl inline-flex gap-0.5 rounded-lg p-[0.1875rem]"><button type="button" aria-pressed="true" class="inline-flex h-7 items-center gap-1.5 rounded-md border-0 px-3 text-[0.8125rem] font-medium transition-colors in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:whitespace-nowrap bg-background text-foreground shadow-card in-data-[scale=deck]:bg-card in-data-[scale=deck]:text-fg in-data-[scale=deck]:ring-border in-data-[scale=deck]:ring-1">Tiếng Việt</button><button type="button" aria-pressed="false" class="inline-flex h-7 items-center gap-1.5 rounded-md border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:whitespace-nowrap text-muted-foreground in-data-[scale=deck]:text-muted-fg">English</button></div>`;
const R3_LARGE = `<div role="group" aria-label="Mục" class="bg-muted in-data-[scale=deck]:rounded-ctl inline-flex gap-0.5 rounded-lg p-[0.1875rem] max-w-full self-start overflow-x-auto [&amp;>button]:outline-offset-1!"><button type="button" aria-pressed="false" class="inline-flex items-center gap-1.5 rounded-md border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:whitespace-nowrap h-8 in-data-[scale=deck]:h-8 text-muted-foreground in-data-[scale=deck]:text-muted-fg">Hồ sơ</button><button type="button" aria-pressed="true" class="inline-flex items-center gap-1.5 rounded-md border-0 px-3 text-[0.8125rem] font-medium transition-colors in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:whitespace-nowrap h-8 in-data-[scale=deck]:h-8 bg-background text-foreground shadow-card in-data-[scale=deck]:bg-card in-data-[scale=deck]:text-fg in-data-[scale=deck]:ring-border in-data-[scale=deck]:ring-1">Đăng nhập</button></div>`;

const LANGUAGES = [
  { value: "vi", label: "Tiếng Việt" },
  { value: "en", label: "English" },
];
const STATUSES: SegmentedOption[] = [
  { value: "live", label: "Đang mở", count: 7 },
  { value: "scheduled", label: "Đã lên lịch", count: 0 },
  { value: "closed", label: "Đã đóng", count: 120 },
  { value: "draft", label: "Bản nháp" },
];
const SOURCES: SegmentedOption[] = [
  { value: "file", label: "Tải tệp lên", icon: FileUp },
  { value: "paste", label: "Dán văn bản", icon: ClipboardPaste },
];

afterEach(() => {
  vi.restoreAllMocks();
});

describe("the segmented control R3's screens were measured on", () => {
  it("renders the default size byte for byte as before", () => {
    const { container } = render(
      <Segmented label="Ngôn ngữ" value="vi" onChange={() => {}} options={LANGUAGES} />,
    );
    expect(container.innerHTML).toBe(R3_DEFAULT);
  });

  it("renders the large size with a caller's classes byte for byte as before", () => {
    const { container } = render(
      <Segmented
        label="Mục"
        size="lg"
        value="b"
        className="max-w-full self-start overflow-x-auto [&>button]:outline-offset-1!"
        onChange={() => {}}
        options={[
          { value: "a", label: "Hồ sơ" },
          { value: "b", label: "Đăng nhập" },
        ]}
      />,
    );
    expect(container.innerHTML).toBe(R3_LARGE);
  });

  it("renders the same markup on a deck surface", () => {
    const { container } = render(
      <DeckScale>
        <Segmented
          label="Ngôn ngữ"
          value="vi"
          onChange={() => {}}
          options={LANGUAGES}
        />
      </DeckScale>,
    );
    expect(container.firstElementChild!.innerHTML).toBe(R3_DEFAULT);
  });
});

function Statuses({ onChange }: Readonly<{ onChange: (value: string) => void }>) {
  const [value, setValue] = useState("live");
  return (
    <Segmented
      label="Trạng thái"
      value={value}
      options={STATUSES}
      onChange={(next) => {
        onChange(next);
        setValue(next);
      }}
    />
  );
}

describe("which option is on", () => {
  it("is a group of buttons with exactly the chosen one pressed", () => {
    render(<Statuses onChange={() => {}} />);
    const group = screen.getByRole("group", { name: "Trạng thái" });
    const buttons = [...group.querySelectorAll("button")];
    expect(buttons.map((button) => button.getAttribute("aria-pressed"))).toEqual([
      "true",
      "false",
      "false",
      "false",
    ]);
    expect(buttons.every((button) => button.type === "button")).toBe(true);
    expect(screen.queryByRole("tablist")).toBeNull();
    expect(screen.queryByRole("tab")).toBeNull();
  });

  it("moves the pressed state to the option that is clicked", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Statuses onChange={onChange} />);
    await user.click(screen.getByRole("button", { name: "Đã đóng 120" }));
    expect(onChange).toHaveBeenCalledWith("closed");
    expect(screen.getByRole("button", { name: "Đã đóng 120" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "Đang mở 7" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });

  it("reports a click on the option that is already pressed, as it always has", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Statuses onChange={onChange} />);
    await user.click(screen.getByRole("button", { name: "Đang mở 7" }));
    await user.click(screen.getByRole("button", { name: "Đang mở 7" }));
    expect(onChange.mock.calls).toEqual([["live"], ["live"]]);
  });

  it("draws the pressed option on the card and the others muted", () => {
    render(<Statuses onChange={() => {}} />);
    const on = screen.getByRole("button", { name: "Đang mở 7" });
    expect(plain(on)).toEqual(
      expect.arrayContaining(["bg-background", "text-foreground", "shadow-card"]),
    );
    expect(inside(on)).toEqual(
      expect.arrayContaining(["bg-card", "text-fg", "ring-1", "ring-border"]),
    );
    const off = screen.getByRole("button", { name: "Đã lên lịch 0" });
    expect(plain(off)).toContain("text-muted-foreground");
    expect(inside(off)).toContain("text-muted-fg");
    expect(inside(off)).not.toContain("bg-card");
  });
});

describe("an option's count", () => {
  it("is drawn on every option that has one, at zero too", () => {
    render(<Statuses onChange={() => {}} />);
    for (const [name, count] of [
      ["Đang mở 7", "7"],
      ["Đã lên lịch 0", "0"],
      ["Đã đóng 120", "120"],
    ] as const) {
      const badge = screen.getByRole("button", { name }).querySelector("span")!;
      expect(badge).toHaveTextContent(count);
      expect(classes(badge)).toEqual([
        "bg-primary",
        "text-primary-fg",
        "text-2xs",
        "inline-flex",
        "h-4.5",
        "min-w-4.5",
        "items-center",
        "justify-center",
        "rounded-full",
        "px-1.25",
        "leading-none",
        "font-semibold",
        "tabular-nums",
      ]);
      expect(badge).not.toHaveAttribute("aria-hidden");
    }
  });

  it("is part of the button's name as a number and nothing else", () => {
    render(<Statuses onChange={() => {}} />);
    const names = [
      ...screen.getByRole("group", { name: "Trạng thái" }).querySelectorAll("button"),
    ].map((button) => button.textContent);
    expect(names).toEqual(["Đang mở 7", "Đã lên lịch 0", "Đã đóng 120", "Bản nháp"]);
  });

  it("is absent from an option that has none", () => {
    render(<Statuses onChange={() => {}} />);
    const draft = screen.getByRole("button", { name: "Bản nháp" });
    expect(draft.querySelector("span")).toBeNull();
    expect(draft.childNodes).toHaveLength(1);
  });
});

describe("an option's icon", () => {
  it("comes before the label, hidden from the name, at the large row's 15px and 7px gap", () => {
    render(
      <Segmented
        label="Nguồn"
        size="lg"
        value="file"
        options={SOURCES}
        onChange={() => {}}
      />,
    );
    const button = screen.getByRole("button", { name: "Tải tệp lên" });
    const icon = button.firstElementChild!;
    expect(icon.tagName.toLowerCase()).toBe("svg");
    expect(button.firstChild).toBe(icon);
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(classes(icon)).toEqual(
      expect.arrayContaining(["shrink-0", "size-[0.9375rem]"]),
    );
    expect(plain(button)).toContain("gap-1.75");
    expect(plain(button)).not.toContain("gap-1.5");
    expect(plain(button)).toContain("h-8");
  });

  it("leaves the large row's gap alone for an option without one", () => {
    render(
      <Segmented
        label="Nguồn"
        size="lg"
        value="file"
        options={[SOURCES[0]!, { value: "paste", label: "Dán văn bản" }]}
        onChange={() => {}}
      />,
    );
    const bare = screen.getByRole("button", { name: "Dán văn bản" });
    expect(bare.querySelector("svg")).toBeNull();
    expect(plain(bare)).toContain("gap-1.5");
    expect(plain(bare)).not.toContain("gap-1.75");
  });

  it.each([
    ["default", "size-[0.9375rem]", "gap-1.5"],
    ["sm", "size-[0.8125rem]", "gap-1.5"],
    ["xs", "size-[0.8125rem]", "gap-1.25"],
  ] as const)("keeps the %s row's own icon size and gap", (size, icon, gap) => {
    render(
      <Segmented
        label="Nguồn"
        size={size}
        value="file"
        options={SOURCES}
        onChange={() => {}}
      />,
    );
    const button = screen.getByRole("button", { name: "Tải tệp lên" });
    expect(classes(button.firstElementChild!)).toContain(icon);
    expect(plain(button).filter((c) => c.startsWith("gap-"))).toEqual([gap]);
  });
});

describe("the two small sizes", () => {
  it("draws the 28px row at 12.5px with 10px of padding", () => {
    render(
      <Segmented
        label="Hiển thị"
        size="sm"
        value="vi"
        options={LANGUAGES}
        onChange={() => {}}
      />,
    );
    const button = screen.getByRole("button", { name: "Tiếng Việt" });
    expect(plain(button)).toEqual(
      expect.arrayContaining(["h-7", "px-2.5", "text-meta", "gap-1.5", "rounded-md"]),
    );
    expect(plain(button)).not.toContain("px-3");
    expect(plain(button)).not.toContain("text-[0.8125rem]");
    expect(inside(button).filter((c) => c.startsWith("h-"))).toEqual(["h-7"]);
    expect(inside(button)).toContain("rounded-seg");
    expect(inside(screen.getByRole("group", { name: "Hiển thị" }))).toEqual([
      "rounded-ctl",
    ]);
  });

  it("draws the 26px row at 12px, radius 6 in a radius 8 track, with a 5px gap", () => {
    render(
      <Segmented
        label="Chế độ soạn"
        size="xs"
        value="vi"
        options={LANGUAGES}
        onChange={() => {}}
      />,
    );
    const button = screen.getByRole("button", { name: "Tiếng Việt" });
    expect(plain(button)).toEqual(
      expect.arrayContaining(["h-6.5", "px-2.5", "text-xs", "gap-1.25", "rounded-md"]),
    );
    expect(plain(button)).not.toContain("h-7");
    expect(plain(button)).not.toContain("text-meta");
    expect(inside(button).filter((c) => c.startsWith("h-"))).toEqual(["h-6.5"]);
    expect(inside(button).filter((c) => c.startsWith("rounded-"))).toEqual([
      "rounded-sm",
    ]);
    const track = screen.getByRole("group", { name: "Chế độ soạn" });
    expect(plain(track)).toContain("rounded-lg");
    expect(inside(track)).toEqual(["rounded-md"]);
  });
});

const SCROLL_TRACK = [
  "max-w-full",
  "[scrollbar-width:none]",
  "overflow-x-auto",
  "[&::-webkit-scrollbar]:hidden",
];
const SCROLL_BUTTON = ["shrink-0", "whitespace-nowrap", "outline-offset-1!"];

function place(element: Element, left: number, right: number) {
  vi.spyOn(element, "getBoundingClientRect").mockReturnValue({
    left,
    right,
    width: right - left,
    top: 0,
    bottom: 30,
    height: 30,
    x: left,
    y: 0,
    toJSON: () => ({}),
  });
}

function Scrolling({ start, from = 0 }: Readonly<{ start: string; from?: number }>) {
  const [value, setValue] = useState(start);
  return (
    <div
      ref={(wrapper) => {
        const track = wrapper?.firstElementChild;
        if (!track) return;
        if (track.scrollLeft === 0) track.scrollLeft = from;
        place(track, 100, 300);
        const [live, scheduled, closed, draft] = track.querySelectorAll("button");
        place(live!, 40, 130);
        place(scheduled!, 132, 250);
        place(closed!, 252, 340);
        place(draft!, 342, 430);
      }}
    >
      <Segmented
        label="Trạng thái"
        scroll
        value={value}
        options={STATUSES}
        onChange={setValue}
      />
    </div>
  );
}

describe("a track that scrolls", () => {
  it("has no scroll classes unless asked", () => {
    render(<Statuses onChange={() => {}} />);
    const track = screen.getByRole("group", { name: "Trạng thái" });
    for (const c of SCROLL_TRACK) expect(classes(track)).not.toContain(c);
    for (const button of track.querySelectorAll("button"))
      for (const c of SCROLL_BUTTON) expect(classes(button)).not.toContain(c);
  });

  it("scrolls sideways inside itself with no scrollbar when asked", () => {
    render(
      <Segmented
        label="Trạng thái"
        scroll
        value="live"
        options={STATUSES}
        onChange={() => {}}
      />,
    );
    const track = screen.getByRole("group", { name: "Trạng thái" });
    expect(classes(track)).toEqual(expect.arrayContaining(SCROLL_TRACK));
    for (const button of track.querySelectorAll("button"))
      expect(classes(button)).toEqual(expect.arrayContaining(SCROLL_BUTTON));
  });

  it("brings a pressed option that starts out of view to the left into view", () => {
    render(<Scrolling start="live" from={80} />);
    const track = screen.getByRole("group", { name: "Trạng thái" });
    expect(track.scrollLeft).toBe(17);
  });

  it("leaves a pressed option that is in view where it is", () => {
    render(<Scrolling start="scheduled" from={80} />);
    expect(screen.getByRole("group", { name: "Trạng thái" }).scrollLeft).toBe(80);
  });

  it("brings a newly pressed option beyond the right edge into view", async () => {
    const user = userEvent.setup();
    render(<Scrolling start="scheduled" />);
    const track = screen.getByRole("group", { name: "Trạng thái" });
    expect(track.scrollLeft).toBe(0);
    await user.click(screen.getByRole("button", { name: "Bản nháp" }));
    expect(track.scrollLeft).toBe(133);
  });

  it("brings a newly pressed option cut by the right edge fully into view", async () => {
    const user = userEvent.setup();
    render(<Scrolling start="scheduled" />);
    const track = screen.getByRole("group", { name: "Trạng thái" });
    await user.click(screen.getByRole("button", { name: "Đã đóng 120" }));
    expect(track.scrollLeft).toBe(43);
  });

  it("leaves the track where it is without the scroll option", async () => {
    const user = userEvent.setup();
    const view = render(<Statuses onChange={() => {}} />);
    const track = screen.getByRole("group", { name: "Trạng thái" });
    place(track, 100, 300);
    place(screen.getByRole("button", { name: "Bản nháp" }), 342, 430);
    await user.click(screen.getByRole("button", { name: "Bản nháp" }));
    expect(track.scrollLeft).toBe(0);
    view.unmount();
  });
});
