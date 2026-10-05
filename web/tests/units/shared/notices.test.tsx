import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FileText, ShieldCheck } from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import { EventList, type EventListItem } from "@/components/shared/EventList";
import { IconTile } from "@/components/shared/IconTile";
import { LockNotice } from "@/components/shared/LockNotice";
import i18n from "@/lib/i18n";

describe("IconTile", () => {
  it("is decoration: hidden from assistive technology, with its icon", () => {
    const { container } = render(<IconTile icon={FileText} />);
    const tile = container.firstElementChild!;
    expect(tile).toHaveAttribute("aria-hidden", "true");
    expect(tile.querySelector("svg.lucide-file-text")).not.toBeNull();
    expect(tile.tagName).toBe("SPAN");
  });

  it.each([
    [28, "size-7", "rounded-seg", "[&>svg]:size-[0.9375rem]"],
    [30, "size-7.5", "rounded-[0.5rem]", "[&>svg]:size-[0.9375rem]"],
    [32, "size-8", "rounded-[0.5rem]", "[&>svg]:size-4"],
    [34, "size-8.5", "rounded-[0.5rem]", "[&>svg]:size-4"],
    [36, "size-9", "rounded-ctl", "[&>svg]:size-[1.0625rem]"],
    [40, "size-10", "rounded-[0.625rem]", "[&>svg]:size-[1.1875rem]"],
    [42, "size-10.5", "rounded-[0.6875rem]", "[&>svg]:size-[1.3125rem]"],
    [44, "size-11", "rounded-xl", "[&>svg]:size-5"],
  ] as const)(
    "at %ipx has the deck's box, radius and icon",
    (size, box, radius, icon) => {
      const { container } = render(<IconTile icon={FileText} size={size} />);
      const classes = [...container.firstElementChild!.classList];
      expect(classes.filter((name) => name.startsWith("size-"))).toEqual([box]);
      expect(classes.filter((name) => name.startsWith("rounded-"))).toEqual([radius]);
      expect(classes.filter((name) => name.startsWith("[&>svg]:size-"))).toEqual([
        icon,
      ]);
    },
  );

  it("is 32px and neutral by default", () => {
    const { container } = render(<IconTile icon={FileText} />);
    expect(container.firstElementChild).toHaveClass(
      "size-8",
      "rounded-[0.5rem]",
      "[&>svg]:size-4",
      "bg-muted",
      "text-fg",
    );
  });

  it.each([
    ["neutral", "bg-muted", "text-fg"],
    ["info", "bg-info-soft", "text-info-ink"],
    ["success", "bg-success-soft", "text-success-ink"],
    ["warning", "bg-warning-soft", "text-warning-ink"],
    ["danger", "bg-danger-soft", "text-danger-ink"],
    ["accent", "bg-brand-soft", "text-brand-ink"],
  ] as const)("in the %s tone is %s with %s", (tone, fill, ink) => {
    const { container } = render(<IconTile icon={FileText} tone={tone} />);
    const classes = [...container.firstElementChild!.classList];
    expect(classes.filter((name) => name.startsWith("bg-"))).toEqual([fill]);
    expect(classes.filter((name) => name.startsWith("text-"))).toEqual([ink]);
  });

  it("lets a caller restate the radius and the icon's size", () => {
    const { container } = render(
      <IconTile
        icon={FileText}
        size={36}
        className="rounded-[0.625rem] [&>svg]:size-4.5"
      />,
    );
    const tile = container.firstElementChild!;
    expect(tile).toHaveClass("size-9", "rounded-[0.625rem]", "[&>svg]:size-4.5");
    expect(tile).not.toHaveClass("rounded-ctl");
    expect(tile).not.toHaveClass("[&>svg]:size-[1.0625rem]");
  });
});

const LEAVE = "Bạn có thể rời trang này. Việc xử lý vẫn tiếp tục.";

describe("Callout", () => {
  it("is a static note with no role", () => {
    const { container } = render(<Callout>{LEAVE}</Callout>);
    const note = container.firstElementChild!;
    expect(note).toHaveTextContent(LEAVE);
    expect(note).not.toHaveAttribute("role");
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("is an alert when it announces", () => {
    render(
      <Callout tone="warning" size="sm" announce>
        Hãy chọn ít nhất hai đáp án đúng.
      </Callout>,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Hãy chọn ít nhất hai đáp án đúng.",
    );
  });

  it("opens with a bold lead, in line before the text", () => {
    const { container } = render(
      <Callout lead="Nội dung của bạn đi đâu.">Tệp được xử lý trên máy chủ.</Callout>,
    );
    const lead = screen.getByText("Nội dung của bạn đi đâu.");
    expect(lead.tagName).toBe("STRONG");
    expect(lead).toHaveClass("font-semibold");
    expect(container.firstElementChild!.textContent).toBe(
      "Nội dung của bạn đi đâu. Tệp được xử lý trên máy chủ.",
    );
  });

  it("draws no lead when it has none", () => {
    const { container } = render(<Callout>{LEAVE}</Callout>);
    expect(container.querySelector("strong")).toBeNull();
    expect(container.firstElementChild!.textContent).toBe(LEAVE);
  });

  it("lays its actions out after the text, as items of a row that wraps", () => {
    const { container } = render(
      <Callout
        tone="warning"
        lead="Bản nháp có thay đổi chưa xuất bản."
        actions={
          <>
            <button type="button">Xem bản nháp</button>
            <button type="button">Xuất bản phiên bản 4</button>
          </>
        }
      >
        Bài giao mới nhận phiên bản 3.
      </Callout>,
    );
    const note = container.firstElementChild!;
    const text = screen.getByText("Bài giao mới nhận phiên bản 3.", { exact: false });
    const [review, publish] = screen.getAllByRole("button");
    expect(note).toHaveClass("flex-wrap");
    expect(text).toHaveClass("flex-[1_1_17.5rem]");
    expect(review!.parentElement).toBe(note);
    expect(publish!.parentElement).toBe(note);
    expect(
      text.compareDocumentPosition(review!) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(review).toHaveTextContent("Xem bản nháp");
    expect(publish).toHaveTextContent("Xuất bản phiên bản 4");
  });

  it("does not wrap and does not stretch its text without actions", () => {
    const { container } = render(<Callout>{LEAVE}</Callout>);
    expect(container.firstElementChild).not.toHaveClass("flex-wrap");
    expect(screen.getByText(LEAVE).className).toBe("min-w-0");
  });

  it.each([null, false] as const)(
    "does not wrap or stretch its text when actions is %s",
    (actions) => {
      const { container } = render(<Callout actions={actions}>{LEAVE}</Callout>);
      expect(container.firstElementChild).not.toHaveClass("flex-wrap");
      expect(screen.getByText(LEAVE).className).toBe("min-w-0");
    },
  );

  it("keeps the 280px text basis when it has actions and a dismiss button", () => {
    render(
      <Callout
        actions={<button type="button">Xem bản nháp</button>}
        onDismiss={vi.fn()}
      >
        {LEAVE}
      </Callout>,
    );
    expect(screen.getByText(LEAVE)).toHaveClass("flex-[1_1_17.5rem]");
    const [action, dismiss] = screen.getAllByRole("button");
    expect(action).toHaveTextContent("Xem bản nháp");
    expect(dismiss).toHaveAccessibleName("Ẩn thông báo");
  });

  it("has a dismiss button, named in Vietnamese, that calls onDismiss", async () => {
    const user = userEvent.setup();
    const onDismiss = vi.fn();
    render(
      <Callout tone="info" size="md" onDismiss={onDismiss}>
        Nên duyệt trên máy tính bảng hoặc máy tính.
      </Callout>,
    );
    const dismiss = screen.getByRole("button", { name: "Ẩn thông báo" });
    expect(dismiss).toHaveAttribute("type", "button");
    expect(dismiss).toHaveClass("size-6", "rounded-[0.375rem]");
    expect(dismiss.querySelector("svg.lucide-x")).toHaveClass("size-3.5");
    expect(onDismiss).not.toHaveBeenCalled();
    await user.click(dismiss);
    expect(onDismiss).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Nên duyệt trên máy tính bảng hoặc máy tính.")).toHaveClass(
      "flex-1",
    );
  });

  it.each([
    [
      "lg",
      "px-3.5",
      "py-3",
      "gap-2.5",
      "rounded-[0.625rem]",
      "text-sm",
      "[&>svg]:size-4",
    ],
    [
      "md",
      "px-3",
      "py-2.5",
      "gap-2",
      "rounded-[0.5rem]",
      "text-sm",
      "[&>svg]:size-[0.9375rem]",
    ],
    [
      "sm",
      "px-2.5",
      "py-2",
      "gap-2",
      "rounded-[0.5rem]",
      "text-meta",
      "[&>svg]:size-3.5",
    ],
  ] as const)(
    "at %s has the deck's padding, gap, radius, type and icon",
    (size, ...own) => {
      const { container } = render(<Callout size={size}>{LEAVE}</Callout>);
      const classes = [...container.firstElementChild!.classList];
      const [px, py, gap, radius, type, icon] = own;
      expect(classes.filter((name) => name.startsWith("px-"))).toEqual([px]);
      expect(classes.filter((name) => name.startsWith("py-"))).toEqual([py]);
      expect(classes.filter((name) => name.startsWith("gap-"))).toEqual([gap]);
      expect(classes.filter((name) => name.startsWith("rounded-"))).toEqual([radius]);
      expect(classes.filter((name) => /^text-(sm|meta)$/.test(name))).toEqual([type]);
      expect(classes.filter((name) => name.startsWith("[&>svg]:size-"))).toEqual([
        icon,
      ]);
    },
  );

  it("is the large note on a 1.55 line by default", () => {
    const { container } = render(<Callout>{LEAVE}</Callout>);
    expect(container.firstElementChild).toHaveClass(
      "px-3.5",
      "py-3",
      "leading-[1.55]",
      "items-start",
      "[&>svg]:mt-0.5",
    );
  });

  it.each([
    ["neutral", "bg-muted", "[&>svg]:text-fg", "lucide-info"],
    ["info", "bg-info-soft", "[&>svg]:text-info-ink", "lucide-info"],
    ["success", "bg-success-soft", "[&>svg]:text-success-ink", "lucide-info"],
    ["warning", "bg-warning-soft", "[&>svg]:text-warning-ink", "lucide-triangle-alert"],
    ["danger", "bg-danger-soft", "[&>svg]:text-danger-ink", "lucide-triangle-alert"],
  ] as const)(
    "in the %s tone is %s, the icon %s, the text in --fg",
    (tone, fill, ink, glyph) => {
      const { container } = render(<Callout tone={tone}>{LEAVE}</Callout>);
      const note = container.firstElementChild!;
      const classes = [...note.classList];
      expect(classes.filter((name) => name.startsWith("bg-"))).toEqual([fill]);
      expect(classes.filter((name) => name.startsWith("[&>svg]:text-"))).toEqual([ink]);
      expect(note).toHaveClass("text-fg");
      const icon = note.querySelector(":scope > svg")!;
      expect(icon).toHaveClass(glyph);
      expect(icon).toHaveAttribute("aria-hidden", "true");
    },
  );

  it("is neutral unless given a tone", () => {
    const { container } = render(<Callout>{LEAVE}</Callout>);
    expect(container.firstElementChild).toHaveClass("bg-muted", "[&>svg]:text-fg");
    expect(container.querySelector("svg")).toHaveClass("lucide-info");
  });

  it("takes an icon in place of its tone's", () => {
    const { container } = render(<Callout icon={ShieldCheck}>{LEAVE}</Callout>);
    const icons = container.querySelectorAll("svg");
    expect(icons).toHaveLength(1);
    expect(icons[0]).toHaveClass("lucide-shield-check");
  });

  it("lets a screen restate the gap, the radius and the icon's size and margin", () => {
    const { container } = render(
      <Callout className="gap-3 rounded-xl [&>svg]:mt-px [&>svg]:size-[1.0625rem]">
        {LEAVE}
      </Callout>,
    );
    const note = container.firstElementChild!;
    expect(note).toHaveClass(
      "gap-3",
      "rounded-xl",
      "[&>svg]:mt-px",
      "[&>svg]:size-[1.0625rem]",
    );
    for (const replaced of [
      "gap-2.5",
      "rounded-[0.625rem]",
      "[&>svg]:mt-0.5",
      "[&>svg]:size-4",
    ])
      expect(note).not.toHaveClass(replaced);
  });
});

const REASON = "Khoá khi học viên đang làm bài";

describe("LockNotice", () => {
  it("shows a lock and the word Locked", () => {
    render(<LockNotice reason={REASON} />);
    const notice = screen.getByTitle(REASON);
    expect(notice).toHaveTextContent(/^Đã khoá/);
    expect(notice.querySelector("svg.lucide-lock")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
    expect(notice).toHaveClass("text-muted-fg", "text-xs", "whitespace-nowrap");
    expect(notice.className).not.toMatch(/border|dashed|bg-/);
  });

  it("gives the reason as its title and as text a screen reader reaches", () => {
    render(<LockNotice reason={REASON} />);
    const notice = screen.getByTitle(REASON);
    expect(notice).toHaveAttribute("title", REASON);
    expect(notice).toHaveTextContent(`Đã khoá ${REASON}`);
    const spoken = within(notice).getByText(REASON);
    expect(spoken).toHaveClass("sr-only");
    expect(spoken).not.toHaveAttribute("aria-hidden");
  });

  it("positions itself, so the hidden reason scrolls with it and never lengthens the page", () => {
    render(<LockNotice reason={REASON} />);
    const notice = screen.getByTitle(REASON);
    expect(within(notice).getByText(REASON).parentElement).toBe(notice);
    expect(notice).toHaveClass("relative");
  });

  it("takes another word in place of Locked", () => {
    render(<LockNotice reason={REASON} label="Đã chốt" />);
    const notice = screen.getByTitle(REASON);
    expect(notice).toHaveTextContent(`Đã chốt ${REASON}`);
    expect(notice).not.toHaveTextContent("Đã khoá");
  });

  it("is text, not a control: it takes no tab stop", async () => {
    const user = userEvent.setup();
    const { container } = render(<LockNotice reason={REASON} />);
    expect(container.querySelector("button, a, input, [tabindex], [role]")).toBeNull();
    await user.tab();
    expect(document.body).toHaveFocus();
  });
});

const FLAGGED: readonly EventListItem[] = [
  { key: "start", tone: "info", text: "Bắt đầu làm bài", time: "19:22" },
  { key: "fs-1", tone: "danger", text: "Rời toàn màn hình · 14 giây", time: "19:41" },
  { key: "tab", tone: "warning", text: "Chuyển tab · 32 giây", time: "19:58" },
  { key: "save", tone: "neutral", text: "Đã tự lưu câu trả lời", time: "mỗi 10 giây" },
  { key: "submit", tone: "success", text: "Đã nộp bài", time: "20:14" },
];

const TIMELINE = "Dòng thời gian của bài làm";

describe("EventList", () => {
  it("is a list named by its label, one item per event, in order", () => {
    render(<EventList label={TIMELINE} items={FLAGGED} />);
    const list = screen.getByRole("list", { name: TIMELINE });
    expect(list.tagName).toBe("OL");
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(5);
    expect(
      items.map((item) => {
        const [text, time] = [...item.lastElementChild!.children];
        return [text!.textContent, time!.textContent];
      }),
    ).toEqual(FLAGGED.map((event) => [event.text, event.time]));
  });

  it("draws each tone's dot, hidden from assistive technology", () => {
    render(<EventList label={TIMELINE} items={FLAGGED} />);
    const dots = screen
      .getAllByRole("listitem")
      .map((item) => item.firstElementChild as HTMLElement);
    for (const dot of dots) {
      expect(dot).toHaveAttribute("aria-hidden", "true");
      expect(dot).toHaveClass("size-2.5", "rounded-full", "ring-3", "ring-card");
      expect(dot).toBeEmptyDOMElement();
    }
    expect(
      dots.map((dot) => [...dot.classList].filter((name) => name.startsWith("bg-"))),
    ).toEqual([
      ["bg-info"],
      ["bg-danger"],
      ["bg-warning"],
      ["bg-border"],
      ["bg-success"],
    ]);
  });

  it("writes the time small and muted under the text", () => {
    render(<EventList label={TIMELINE} items={FLAGGED} />);
    expect(screen.getByText("19:22")).toHaveClass("text-muted-fg", "text-xs");
    expect(screen.getByText("Bắt đầu làm bài").parentElement).toHaveClass("text-sm");
  });

  it("keeps the deck's space under every item, the last included", () => {
    render(<EventList label={TIMELINE} items={FLAGGED} />);
    for (const item of screen.getAllByRole("listitem"))
      expect(item).toHaveClass("pb-3.5");
  });

  it("takes a class from the caller on the list", () => {
    render(<EventList label={TIMELINE} items={FLAGGED} className="mt-2" />);
    expect(screen.getByRole("list", { name: TIMELINE })).toHaveClass("mt-2");
  });

  it("takes a node as an event's text", () => {
    render(
      <EventList
        label={TIMELINE}
        items={[
          {
            key: "start",
            tone: "info",
            text: (
              <>
                Bắt đầu <em>lần 2</em>
              </>
            ),
            time: "19:22",
          },
        ]}
      />,
    );
    const emphasis = screen.getByText("lần 2");
    expect(emphasis.tagName).toBe("EM");
    expect(emphasis.parentElement).toHaveTextContent("Bắt đầu lần 2");
    expect(screen.getByRole("listitem")).toContainElement(emphasis);
  });

  it("renders nothing for an empty list", () => {
    const { container } = render(<EventList label={TIMELINE} items={[]} />);
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByRole("list")).toBeNull();
  });
});

describe("the display strings in English", () => {
  afterEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("writes the deck's words", async () => {
    await i18n.changeLanguage("en");
    const reason = "Locked while students are taking the test";
    render(
      <>
        <LockNotice reason={reason} />
        <Callout onDismiss={vi.fn()}>You can leave this page.</Callout>
      </>,
    );
    expect(screen.getByTitle(reason)).toHaveTextContent(/^Locked /);
    expect(screen.getByRole("button", { name: "Dismiss" })).toBeInTheDocument();
  });
});
