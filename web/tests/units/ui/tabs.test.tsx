import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const classes = (element: Element) =>
  (element.getAttribute("class") ?? "").split(" ").filter(Boolean);

const LIST = [
  "flex",
  "gap-5",
  "overflow-x-auto",
  "rounded-none",
  "border-b",
  "bg-transparent",
  "p-0",
];
const TRIGGER = [
  "inline-flex",
  "items-center",
  "gap-1.5",
  "border-0",
  "bg-transparent",
  "font-medium",
  "transition-colors",
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

describe("the tabs' row", () => {
  it("is the deck's underlined row: 20px apart, a line beneath, scrolling sideways", () => {
    render(<Detail />);
    const list = screen.getByRole("tablist");
    expect(classes(list)).toEqual(LIST);
    for (const replaced of [
      "inline-flex",
      "bg-muted",
      "gap-0.5",
      "rounded-lg",
      "p-[0.1875rem]",
    ])
      expect(classes(list)).not.toContain(replaced);
  });

  it("draws the selected tab in the foreground colour over a 2px line", () => {
    render(<Detail />);
    const on = tab("Học viên");
    const off = tab("Câu hỏi");
    expect(on).toHaveAttribute("data-state", "active");
    expect(on).toHaveAttribute("aria-selected", "true");
    expect(off).toHaveAttribute("data-state", "inactive");
    for (const trigger of [on, off]) {
      expect(classes(trigger)).toEqual(TRIGGER);
      for (const replaced of [
        "text-muted-foreground",
        "data-[state=active]:bg-background",
        "data-[state=active]:text-foreground",
        "data-[state=active]:shadow-card",
        "h-7",
        "rounded-md",
        "px-3",
        "text-[0.8125rem]",
      ])
        expect(classes(trigger)).not.toContain(replaced);
    }
  });

  it("keeps a focus ring of its own, drawn inside the tab, and never removes the outline", () => {
    render(<Detail />);
    for (const name of ["Học viên", "Câu hỏi", "Cài đặt"]) {
      const outline = classes(tab(name)).filter((c) => c.includes("outline"));
      expect(outline).toEqual(["-outline-offset-2!"]);
    }
  });
});

describe("tabs from the keyboard", () => {
  it("move with the arrow keys and show the tab's panel", async () => {
    const user = userEvent.setup();
    render(<Detail />);
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
      <Tabs defaultValue="students">
        <TabsList
          aria-label="Bài giao"
          onFocus={(event) => seen.push(event.target.textContent)}
        >
          <TabsTrigger value="students">Học viên</TabsTrigger>
          <TabsTrigger value="questions">Câu hỏi</TabsTrigger>
          <TabsTrigger value="settings">Cài đặt</TabsTrigger>
        </TabsList>
      </Tabs>,
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
