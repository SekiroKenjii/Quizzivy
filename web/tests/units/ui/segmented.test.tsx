import { afterEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ClipboardPaste, FileUp } from "lucide-react";
import { useState } from "react";
import { Segmented, type SegmentedOption } from "@/components/ui/segmented";
import i18n from "@/lib/i18n";

const classes = (element: Element) =>
  (element.getAttribute("class") ?? "").split(" ").filter(Boolean);

const DEFAULT_MARKUP = `<div role="group" aria-label="Ngôn ngữ" class="bg-muted rounded-ctl inline-flex gap-0.5 p-[0.1875rem]"><button type="button" aria-pressed="true" class="inline-flex items-center gap-1.5 border-0 px-3 text-[0.8125rem] font-medium transition-colors rounded-seg h-7.5 whitespace-nowrap shadow-card bg-card text-fg ring-border ring-1">Tiếng Việt</button><button type="button" aria-pressed="false" class="inline-flex items-center gap-1.5 border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors rounded-seg h-7.5 whitespace-nowrap text-muted-fg">English</button></div>`;
const LARGE_MARKUP = `<div role="group" aria-label="Mục" class="bg-muted rounded-ctl inline-flex gap-0.5 p-[0.1875rem] max-w-full self-start overflow-x-auto [&amp;>button]:outline-offset-1!"><button type="button" aria-pressed="false" class="inline-flex items-center gap-1.5 border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors rounded-seg whitespace-nowrap h-8 text-muted-fg">Hồ sơ</button><button type="button" aria-pressed="true" class="inline-flex items-center gap-1.5 border-0 px-3 text-[0.8125rem] font-medium transition-colors rounded-seg whitespace-nowrap h-8 shadow-card bg-card text-fg ring-border ring-1">Đăng nhập</button></div>`;

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

describe("the segmented control's markup", () => {
  it("renders the default size byte for byte", () => {
    const { container } = render(
      <Segmented label="Ngôn ngữ" value="vi" onChange={() => {}} options={LANGUAGES} />,
    );
    expect(container.innerHTML).toBe(DEFAULT_MARKUP);
  });

  it("renders the large size with a caller's classes byte for byte", () => {
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
    expect(container.innerHTML).toBe(LARGE_MARKUP);
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
    expect(classes(on)).toEqual(
      expect.arrayContaining([
        "bg-card",
        "text-fg",
        "ring-1",
        "ring-border",
        "shadow-card",
      ]),
    );
    for (const replaced of ["bg-background", "bg-transparent", "text-foreground"])
      expect(classes(on)).not.toContain(replaced);
    const off = screen.getByRole("button", { name: "Đã lên lịch 0" });
    expect(classes(off)).toContain("text-muted-fg");
    expect(classes(off)).not.toContain("text-muted-foreground");
    expect(classes(off)).not.toContain("bg-card");
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

  it("groups its thousands for the viewer's language", async () => {
    const big: SegmentedOption[] = [{ value: "all", label: "Tất cả", count: 1248 }];
    const { rerender } = render(
      <Segmented label="Nhóm" value="all" options={big} onChange={() => {}} />,
    );
    expect(screen.getByRole("button", { name: "Tất cả 1.248" })).toBeVisible();

    await act(() => i18n.changeLanguage("en"));
    rerender(<Segmented label="Group" value="all" options={big} onChange={() => {}} />);
    expect(screen.getByRole("button", { name: "Tất cả 1,248" })).toBeVisible();
    await act(() => i18n.changeLanguage("vi"));
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
    expect(classes(button)).toContain("gap-1.75");
    expect(classes(button)).not.toContain("gap-1.5");
    expect(classes(button)).toContain("h-8");
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
    expect(classes(bare)).toContain("gap-1.5");
    expect(classes(bare)).not.toContain("gap-1.75");
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
    expect(classes(button).filter((c) => c.startsWith("gap-"))).toEqual([gap]);
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
    expect(classes(button)).toEqual(
      expect.arrayContaining([
        "h-7",
        "px-2.5",
        "text-meta",
        "leading-4",
        "gap-1.5",
        "rounded-seg",
      ]),
    );
    expect(classes(button)).not.toContain("px-3");
    expect(classes(button)).not.toContain("text-[0.8125rem]");
    expect(classes(button)).not.toContain("rounded-md");
    expect(classes(button).filter((c) => c.startsWith("h-"))).toEqual(["h-7"]);
    const track = screen.getByRole("group", { name: "Hiển thị" });
    expect(classes(track).filter((c) => c.startsWith("rounded-"))).toEqual([
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
    expect(classes(button)).toEqual(
      expect.arrayContaining([
        "h-6.5",
        "px-2.5",
        "text-xs",
        "leading-[0.9375rem]",
        "gap-1.25",
      ]),
    );
    expect(classes(button)).not.toContain("h-7");
    expect(classes(button)).not.toContain("text-meta");
    expect(classes(button).filter((c) => c.startsWith("h-"))).toEqual(["h-6.5"]);
    expect(classes(button).filter((c) => c.startsWith("rounded-"))).toEqual([
      "rounded-sm",
    ]);
    const track = screen.getByRole("group", { name: "Chế độ soạn" });
    expect(classes(track).filter((c) => c.startsWith("rounded-"))).toEqual([
      "rounded-md",
    ]);
  });
});

const SCROLL_TRACK = [
  "max-w-full",
  "[scrollbar-width:none]",
  "overflow-x-auto",
  "[&::-webkit-scrollbar]:hidden",
];
const SCROLL_BUTTON = [
  "shrink-0",
  "leading-4",
  "whitespace-nowrap",
  "outline-offset-1!",
];

function place(element: Element, left: number, right: number, moved = () => 0) {
  vi.spyOn(element, "getBoundingClientRect").mockImplementation(() => ({
    left: left - moved(),
    right: right - moved(),
    width: right - left,
    top: 0,
    bottom: 30,
    height: 30,
    x: left - moved(),
    y: 0,
    toJSON: () => ({}),
  }));
}

function Scrolling({ start, from = 0 }: Readonly<{ start: string; from?: number }>) {
  const [value, setValue] = useState(start);
  return (
    <div
      ref={(wrapper) => {
        const track = wrapper?.firstElementChild;
        if (!track) return;
        if (track.scrollLeft === 0) track.scrollLeft = from;
        const moved = () => track.scrollLeft - from;
        place(track, 100, 300);
        const [live, scheduled, closed, draft] = track.querySelectorAll("button");
        place(live!, 40, 130, moved);
        place(scheduled!, 132, 250, moved);
        place(closed!, 252, 340, moved);
        place(draft!, 342, 430, moved);
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
    for (const button of track.querySelectorAll("button")) {
      expect(classes(button)).toContain("whitespace-nowrap");
      for (const c of SCROLL_BUTTON.filter((c) => c !== "whitespace-nowrap"))
        expect(classes(button)).not.toContain(c);
    }
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

  it("brings an option that takes focus while the edge cuts it fully into view", () => {
    render(<Scrolling start="scheduled" />);
    const track = screen.getByRole("group", { name: "Trạng thái" });
    const closed = screen.getByRole("button", { name: "Đã đóng 120" });
    act(() => closed.focus());
    expect(track.scrollLeft).toBe(43);
    expect(closed).toHaveAttribute("aria-pressed", "false");
  });

  it("brings an option that takes focus while the left edge cuts it into view", () => {
    render(<Scrolling start="scheduled" from={80} />);
    const track = screen.getByRole("group", { name: "Trạng thái" });
    expect(track.scrollLeft).toBe(80);
    act(() => screen.getByRole("button", { name: "Đang mở 7" }).focus());
    expect(track.scrollLeft).toBe(17);
  });

  it("leaves an option that takes focus in view where it is", () => {
    render(<Scrolling start="scheduled" />);
    act(() => screen.getByRole("button", { name: "Đã lên lịch 0" }).focus());
    expect(screen.getByRole("group", { name: "Trạng thái" }).scrollLeft).toBe(0);
  });

  it("leaves the track where it is on focus without the scroll option", () => {
    const view = render(<Statuses onChange={() => {}} />);
    const track = screen.getByRole("group", { name: "Trạng thái" });
    place(track, 100, 300);
    place(screen.getByRole("button", { name: "Bản nháp" }), 342, 430);
    act(() => screen.getByRole("button", { name: "Bản nháp" }).focus());
    expect(track.scrollLeft).toBe(0);
    view.unmount();
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

it("adds a wrapping equal-width form row without changing the default", () => {
  render(
    <Segmented
      label="Hàng"
      value="a"
      onChange={() => {}}
      options={[
        { value: "a", label: "Một" },
        { value: "b", label: "Hai" },
      ]}
      fill
    />,
  );
  expect(screen.getByRole("group")).toHaveClass("flex-wrap", "w-full");
  for (const button of screen.getAllByRole("button"))
    expect(button).toHaveClass(
      "flex-1",
      "justify-center",
      "h-7.5",
      "text-meta",
      "px-2.5",
    );
});

it("draws a large form row 32px high with 13px labels, as the wizard's rule groups (F-47)", () => {
  render(
    <Segmented
      label="Hàng"
      value="a"
      onChange={() => {}}
      options={[
        { value: "a", label: "Một" },
        { value: "b", label: "Hai" },
      ]}
      size="lg"
      fill
    />,
  );
  for (const button of screen.getAllByRole("button")) {
    expect(button).toHaveClass("h-8", "px-3", "text-[0.8125rem]");
    expect(button).toHaveClass("flex-1", "justify-center");
    expect(button).not.toHaveClass("h-7.5");
    expect(button).not.toHaveClass("text-meta");
    expect(button).not.toHaveClass("px-2.5");
  }
});

it("disables an option, says why, and leaves the others as they were", async () => {
  const onChange = vi.fn();
  render(
    <>
      <Segmented
        label="Chế độ"
        value="rich"
        onChange={onChange}
        options={[
          { value: "rich", label: "Văn bản định dạng" },
          { value: "markdown", label: "Markdown", disabled: true, describedBy: "why" },
        ]}
      />
      <p id="why">Hãy bỏ các chỗ trống trước.</p>
    </>,
  );
  const markdown = screen.getByRole("button", { name: "Markdown" });
  expect(markdown).toBeDisabled();
  expect(markdown).toHaveAccessibleDescription("Hãy bỏ các chỗ trống trước.");
  expect(markdown).toHaveClass("text-muted-fg/45");
  expect(markdown).not.toHaveClass("opacity-45");
  expect(screen.getByRole("button", { name: "Văn bản định dạng" })).not.toHaveClass(
    "text-muted-fg/45",
  );
  await userEvent.setup().click(markdown);
  expect(onChange).not.toHaveBeenCalled();
});

it("keeps focus on an option while the control is pending and ignores a press", async () => {
  const onChange = vi.fn();
  function Saving() {
    const [value, setValue] = useState("vi");
    const [pending, setPending] = useState(false);
    return (
      <Segmented
        label="Ngôn ngữ"
        value={value}
        pending={pending}
        onChange={(next) => {
          onChange(next);
          setValue(next);
          setPending(true);
        }}
        options={LANGUAGES}
      />
    );
  }
  render(<Saving />);
  const user = userEvent.setup();
  const english = screen.getByRole("button", { name: "English" });
  await user.click(english);
  for (const button of screen.getAllByRole("button")) {
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).not.toBeDisabled();
  }
  expect(english).toHaveFocus();
  await user.keyboard("{Shift>}{Tab}{/Shift}");
  const vietnamese = screen.getByRole("button", { name: "Tiếng Việt" });
  expect(vietnamese).toHaveFocus();
  await user.keyboard("{Enter}");
  expect(vietnamese).toHaveFocus();
  expect(onChange).toHaveBeenCalledTimes(1);
});

it("marks one pending option unavailable and leaves the others choosable", async () => {
  const onChange = vi.fn();
  render(
    <Segmented
      label="Ngôn ngữ"
      value="vi"
      onChange={onChange}
      options={[
        { value: "vi", label: "Tiếng Việt" },
        { value: "en", label: "English", pending: true },
      ]}
    />,
  );
  const user = userEvent.setup();
  const english = screen.getByRole("button", { name: "English" });
  english.focus();
  await user.keyboard("{Enter}");
  expect(english).toHaveFocus();
  expect(english).toHaveAttribute("aria-disabled", "true");
  expect(english).toHaveClass("text-muted-fg/45");
  expect(english).not.toHaveClass("opacity-45");
  expect(onChange).not.toHaveBeenCalled();
  const vietnamese = screen.getByRole("button", { name: "Tiếng Việt" });
  expect(vietnamese).not.toHaveAttribute("aria-disabled");
  await user.click(vietnamese);
  expect(onChange).toHaveBeenCalledWith("vi");
});
