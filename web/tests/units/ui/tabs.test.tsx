import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeckScale } from "@/components/ui/deck-scale";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const INSIDE = "in-data-[scale=deck]:";

const classes = (element: Element) =>
  (element.getAttribute("class") ?? "").split(" ").filter(Boolean);
const plain = (element: Element) =>
  classes(element).filter((c) => !c.includes("scale=deck"));
const inside = (element: Element) =>
  classes(element)
    .filter((c) => c.startsWith(INSIDE))
    .map((c) => c.slice(INSIDE.length));

const TODAY_LIST = [
  "bg-muted",
  "inline-flex",
  "gap-0.5",
  "rounded-lg",
  "p-[0.1875rem]",
];
const TODAY_TRIGGER = [
  "text-muted-foreground",
  "data-[state=active]:bg-background",
  "data-[state=active]:text-foreground",
  "data-[state=active]:shadow-card",
  "inline-flex",
  "h-7",
  "items-center",
  "gap-1.5",
  "rounded-md",
  "border-0",
  "bg-transparent",
  "px-3",
  "text-[0.8125rem]",
  "font-medium",
  "transition-colors",
];

function Detail() {
  return (
    <Tabs defaultValue="students">
      <TabsList aria-label="Bài giao">
        <TabsTrigger value="students">Học viên</TabsTrigger>
        <TabsTrigger value="questions">Câu hỏi</TabsTrigger>
        <TabsTrigger value="settings">Cài đặt</TabsTrigger>
      </TabsList>
      <TabsContent value="students">Danh sách học viên</TabsContent>
      <TabsContent value="questions">Phân tích câu hỏi</TabsContent>
      <TabsContent value="settings">Cài đặt bài giao</TabsContent>
    </Tabs>
  );
}

const tab = (name: string) => screen.getByRole("tab", { name });

function place(element: Element, left: number, right: number, moved = () => 0) {
  vi.spyOn(element, "getBoundingClientRect").mockImplementation(() => ({
    left: left - moved(),
    right: right - moved(),
    width: right - left,
    top: 0,
    bottom: 28,
    height: 28,
    x: left - moved(),
    y: 0,
    toJSON: () => ({}),
  }));
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("tabs off a deck surface", () => {
  it("are today's pills", () => {
    render(<Detail />);
    expect(plain(screen.getByRole("tablist"))).toEqual(TODAY_LIST);
    expect(plain(tab("Học viên"))).toEqual(TODAY_TRIGGER);
    expect(plain(tab("Câu hỏi"))).toEqual(TODAY_TRIGGER);
    expect(screen.getByRole("tablist").closest("[data-scale]")).toBeNull();
  });
});

describe("tabs on a deck surface", () => {
  it("sit in the deck's underlined row: 20px apart, a line beneath, scrolling sideways", () => {
    render(
      <DeckScale>
        <Detail />
      </DeckScale>,
    );
    const list = screen.getByRole("tablist");
    expect(plain(list)).toEqual(TODAY_LIST);
    expect(inside(list)).toEqual([
      "flex",
      "gap-5",
      "overflow-x-auto",
      "rounded-none",
      "border-b",
      "bg-transparent",
      "p-0",
    ]);
  });

  it("draws the selected tab in the foreground colour over a 2px line", () => {
    render(
      <DeckScale>
        <Detail />
      </DeckScale>,
    );
    const on = tab("Học viên");
    const off = tab("Câu hỏi");
    expect(on).toHaveAttribute("data-state", "active");
    expect(on).toHaveAttribute("aria-selected", "true");
    expect(off).toHaveAttribute("data-state", "inactive");
    for (const trigger of [on, off]) {
      expect(plain(trigger)).toEqual(TODAY_TRIGGER);
      expect(inside(trigger)).toEqual([
        "text-muted-fg",
        "text-ui",
        "h-auto",
        "rounded-none",
        "px-0.5",
        "pb-2.5",
        "leading-4.5",
        "whitespace-nowrap",
        "-outline-offset-2!",
        "data-[state=active]:text-fg",
        "data-[state=active]:bg-transparent",
        "data-[state=active]:shadow-[inset_0_-2px_0_var(--fg)]",
      ]);
    }
  });

  it("keeps a focus ring of its own, drawn inside the tab, and never removes the outline", () => {
    render(
      <DeckScale>
        <Detail />
      </DeckScale>,
    );
    for (const name of ["Học viên", "Câu hỏi", "Cài đặt"]) {
      const outline = classes(tab(name)).filter((c) => c.includes("outline"));
      expect(outline).toEqual(["in-data-[scale=deck]:-outline-offset-2!"]);
    }
  });
});

describe("tabs from the keyboard", () => {
  it("move with the arrow keys and show the tab's panel", async () => {
    const user = userEvent.setup();
    render(
      <DeckScale>
        <Detail />
      </DeckScale>,
    );
    expect(screen.getByRole("tabpanel")).toHaveTextContent("Danh sách học viên");
    await user.tab();
    expect(tab("Học viên")).toHaveFocus();

    await user.keyboard("{ArrowRight}");
    expect(tab("Câu hỏi")).toHaveFocus();
    expect(tab("Câu hỏi")).toHaveAttribute("aria-selected", "true");
    expect(tab("Học viên")).toHaveAttribute("aria-selected", "false");
    expect(screen.getByRole("tabpanel")).toHaveTextContent("Phân tích câu hỏi");

    await user.keyboard("{ArrowLeft}");
    expect(tab("Học viên")).toHaveFocus();
    await user.keyboard("{End}");
    expect(tab("Cài đặt")).toHaveFocus();
    expect(screen.getByRole("tabpanel")).toHaveTextContent("Cài đặt bài giao");
    await user.keyboard("{ArrowRight}");
    expect(tab("Học viên")).toHaveFocus();
    await user.keyboard("{Home}");
    expect(tab("Học viên")).toHaveAttribute("aria-selected", "true");
  });

  it("brings a tab that takes focus while the row's edge cuts it into view", async () => {
    const user = userEvent.setup();
    const seen: string[] = [];
    render(
      <DeckScale>
        <Tabs defaultValue="students">
          <TabsList
            aria-label="Bài giao"
            onFocus={(event) => seen.push(event.target.textContent)}
          >
            <TabsTrigger value="students">Học viên</TabsTrigger>
            <TabsTrigger value="questions">Câu hỏi</TabsTrigger>
            <TabsTrigger value="settings">Cài đặt</TabsTrigger>
          </TabsList>
        </Tabs>
      </DeckScale>,
    );
    const list = screen.getByRole("tablist");
    const moved = () => list.scrollLeft;
    place(list, 0, 200);
    place(tab("Học viên"), 0, 90, moved);
    place(tab("Câu hỏi"), 110, 230, moved);
    place(tab("Cài đặt"), 250, 330, moved);

    await user.tab();
    expect(list.scrollLeft).toBe(0);
    await user.keyboard("{ArrowRight}");
    expect(tab("Câu hỏi")).toHaveFocus();
    expect(list.scrollLeft).toBe(30);
    await user.keyboard("{ArrowRight}");
    expect(list.scrollLeft).toBe(130);
    await user.keyboard("{ArrowLeft}{ArrowLeft}");
    expect(tab("Học viên")).toHaveFocus();
    expect(list.scrollLeft).toBe(0);
    expect(seen.slice(-4)).toEqual(["Câu hỏi", "Cài đặt", "Câu hỏi", "Học viên"]);
  });

  it("selects a tab on click", async () => {
    const user = userEvent.setup();
    render(<Detail />);
    await user.click(tab("Cài đặt"));
    expect(tab("Cài đặt")).toHaveAttribute("data-state", "active");
    expect(screen.getByRole("tabpanel")).toHaveTextContent("Cài đặt bài giao");
  });
});
