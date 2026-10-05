import { afterEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { Pager } from "@/components/shared/data/Pager";
import { DeckScale } from "@/components/ui/deck-scale";
import { usePage, usePageSize } from "@/hooks/usePage";
import i18n from "@/lib/i18n";
import { viewport } from "@tests/support/viewport";

type Noun = (count: number) => string;

function nounOf(count: number) {
  if (i18n.language !== "en") return "bài giao";
  return count === 1 ? "assignment" : "assignments";
}

function Screen({
  total,
  noun,
  page: answered,
  sizes,
  frame,
}: Readonly<{
  total: number;
  noun: Noun;
  page: number | undefined;
  sizes: readonly number[] | undefined;
  frame: "card" | "plain" | undefined;
}>) {
  const [page] = usePage();
  const [size] = usePageSize(sizes);
  return (
    <DeckScale>
      <Pager
        page={answered ?? page}
        pageSize={size}
        total={total}
        noun={noun}
        sizes={sizes}
        frame={frame}
      />
    </DeckScale>
  );
}

function renderPager(
  search: string,
  total: number,
  options: {
    page?: number;
    sizes?: readonly number[];
    frame?: "card" | "plain";
  } = {},
) {
  const noun = vi.fn<Noun>(nounOf);
  const router = createMemoryRouter(
    [
      {
        path: "/teacher/assignments",
        element: (
          <Screen
            total={total}
            noun={noun}
            page={options.page}
            sizes={options.sizes}
            frame={options.frame}
          />
        ),
      },
    ],
    { initialEntries: [`/teacher/assignments${search}`] },
  );
  const view = render(<RouterProvider router={router} />);
  return { router, noun, view };
}

const NAMES = {
  first: "Trang đầu",
  previous: "Trang trước",
  next: "Trang sau",
  last: "Trang cuối",
} as const;

const nav = (name = "Phân trang") => screen.getByRole("navigation", { name });
const arrow = (which: keyof typeof NAMES) =>
  within(nav()).getByRole("link", { name: NAMES[which] });
const classes = (element: Element) => element.className.split(" ");
const english = () => act(() => i18n.changeLanguage("en"));

afterEach(async () => {
  vi.unstubAllGlobals();
  await act(() => i18n.changeLanguage("vi"));
});

describe("the range", () => {
  it("reads in Vietnamese with every figure grouped for the locale", () => {
    const { noun } = renderPager("", 1284);
    expect(within(nav()).getByText("1–10 trên 1.284 bài giao")).toBeInTheDocument();
    expect(noun.mock.calls.map(([count]) => count)).toContain(1284);
    expect(noun.mock.calls.every(([count]) => count === 1284)).toBe(true);
  });

  it("groups the positions of a late page too", () => {
    renderPager("?page=101", 1284);
    expect(
      within(nav()).getByText("1.001–1.010 trên 1.284 bài giao"),
    ).toBeInTheDocument();
    expect(within(nav()).getByText("Trang 101 trên 129")).toBeInTheDocument();
  });

  it("reads in English with the deck's words and the noun's own plural", async () => {
    const many = renderPager("", 1284);
    await english();
    const bar = within(nav("Pagination"));
    expect(bar.getByText("1–10 of 1,284 assignments")).toBeInTheDocument();
    expect(bar.getByText("Page 1 of 129")).toBeInTheDocument();
    many.view.unmount();
    renderPager("", 1);
    expect(
      within(nav("Pagination")).getByText("1–1 of 1 assignment"),
    ).toBeInTheDocument();
  });

  it("ends at the total on the last page", () => {
    renderPager("?page=32", 312);
    expect(within(nav()).getByText("311–312 trên 312 bài giao")).toBeInTheDocument();
  });

  it("says there is none for an empty list, asking the noun for zero", async () => {
    const { noun } = renderPager("", 0);
    expect(within(nav()).getByText("Không có bài giao nào")).toBeInTheDocument();
    expect(noun.mock.calls.map(([count]) => count)).toContain(0);
    expect(noun.mock.calls.every(([count]) => count === 0)).toBe(true);
    await english();
    expect(within(nav("Pagination")).getByText("No assignments")).toBeInTheDocument();
  });
});

describe("the page of pages", () => {
  it("counts the pages at the size in use", () => {
    renderPager("", 312);
    expect(within(nav()).getByText("Trang 1 trên 32")).toBeInTheDocument();
  });

  it("shows the last page for a page past the end", () => {
    renderPager("?page=40", 312, { page: 40 });
    expect(within(nav()).getByText("Trang 32 trên 32")).toBeInTheDocument();
    expect(within(nav()).getByText("311–312 trên 312 bài giao")).toBeInTheDocument();
    expect(arrow("next")).toHaveAttribute("aria-disabled", "true");
    expect(arrow("next")).toHaveAttribute("href", "/teacher/assignments?page=32");
  });
});

describe("the four arrows", () => {
  it("link to the first, previous, next and last page, other parameters kept", () => {
    renderPager("?tab=live&page=3", 312);
    expect(arrow("first")).toHaveAttribute("href", "/teacher/assignments?tab=live");
    expect(arrow("previous")).toHaveAttribute(
      "href",
      "/teacher/assignments?tab=live&page=2",
    );
    expect(arrow("next")).toHaveAttribute(
      "href",
      "/teacher/assignments?tab=live&page=4",
    );
    expect(arrow("last")).toHaveAttribute(
      "href",
      "/teacher/assignments?tab=live&page=32",
    );
    for (const which of ["first", "previous", "next", "last"] as const) {
      expect(arrow(which)).not.toHaveAttribute("aria-disabled");
      expect(classes(arrow(which))).not.toContain("opacity-45");
      expect(classes(arrow(which))).not.toContain("pointer-events-none");
      expect(arrow(which).querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    }
  });

  it("keeps the size in the links", () => {
    renderPager("?size=20&page=2", 312);
    expect(arrow("next")).toHaveAttribute(
      "href",
      "/teacher/assignments?size=20&page=3",
    );
    expect(arrow("last")).toHaveAttribute(
      "href",
      "/teacher/assignments?size=20&page=16",
    );
    expect(arrow("first")).toHaveAttribute("href", "/teacher/assignments?size=20");
  });

  it("turn the page through the URL", async () => {
    const { router } = renderPager("?tab=live", 312);
    const user = userEvent.setup();
    await user.click(arrow("next"));
    expect(router.state.location.search).toBe("?tab=live&page=2");
    expect(within(nav()).getByText("11–20 trên 312 bài giao")).toBeInTheDocument();
    expect(within(nav()).getByText("Trang 2 trên 32")).toBeInTheDocument();
    await user.click(arrow("last"));
    expect(router.state.location.search).toBe("?tab=live&page=32");
    await user.click(arrow("previous"));
    expect(router.state.location.search).toBe("?tab=live&page=31");
    await user.click(arrow("first"));
    expect(router.state.location.search).toBe("?tab=live");
  });

  it("go nowhere from the first page: first and previous are off", async () => {
    const { router } = renderPager("?tab=live", 312);
    const user = userEvent.setup();
    for (const which of ["first", "previous"] as const) {
      expect(arrow(which)).toHaveAttribute("aria-disabled", "true");
      expect(arrow(which)).toHaveAttribute("href", "/teacher/assignments?tab=live");
      expect(classes(arrow(which))).toEqual(
        expect.arrayContaining(["pointer-events-none", "opacity-45"]),
      );
      await user.click(arrow(which));
      expect(router.state.location.search).toBe("?tab=live");
      arrow(which).focus();
      await user.keyboard("{Enter}");
      expect(router.state.location.search).toBe("?tab=live");
    }
    expect(arrow("next")).not.toHaveAttribute("aria-disabled");
    expect(arrow("last")).not.toHaveAttribute("aria-disabled");
  });

  it("go nowhere from the last page: next and last are off", async () => {
    const { router } = renderPager("?tab=live&page=32", 312);
    const user = userEvent.setup();
    for (const which of ["next", "last"] as const) {
      expect(arrow(which)).toHaveAttribute("aria-disabled", "true");
      expect(arrow(which)).toHaveAttribute(
        "href",
        "/teacher/assignments?tab=live&page=32",
      );
      expect(classes(arrow(which))).toEqual(
        expect.arrayContaining(["pointer-events-none", "opacity-45"]),
      );
      await user.click(arrow(which));
      expect(router.state.location.search).toBe("?tab=live&page=32");
      arrow(which).focus();
      await user.keyboard("{Enter}");
      expect(router.state.location.search).toBe("?tab=live&page=32");
    }
    expect(arrow("first")).not.toHaveAttribute("aria-disabled");
    expect(arrow("previous")).not.toHaveAttribute("aria-disabled");
  });
});

describe("a list of one page or none", () => {
  it.each([
    ["one page", 3, "1–3 trên 3 bài giao"],
    ["a page exactly full", 10, "1–10 trên 10 bài giao"],
    ["no rows", 0, "Không có bài giao nào"],
  ] as const)("still draws the whole control for %s", (_, total, range) => {
    renderPager("", total);
    expect(within(nav()).getByText(range)).toBeInTheDocument();
    expect(within(nav()).getByText("Trang 1 trên 1")).toBeInTheDocument();
    expect(
      within(nav()).getByRole("combobox", { name: "Số dòng mỗi trang" }),
    ).toBeInTheDocument();
    for (const which of ["first", "previous", "next", "last"] as const) {
      expect(arrow(which)).toHaveAttribute("aria-disabled", "true");
      expect(arrow(which)).toHaveAttribute("href", "/teacher/assignments");
    }
  });
});

describe("rows per page", () => {
  const trigger = () =>
    within(nav()).getByRole("combobox", { name: "Số dòng mỗi trang" });

  it("is our select, named by its visible label, showing the size in use", async () => {
    renderPager("?size=30", 312);
    expect(within(nav()).getByText("Số dòng mỗi trang")).toBeInTheDocument();
    expect(nav().querySelector("select")).toBeNull();
    expect(trigger()).toHaveTextContent("30");
    await userEvent.setup().click(trigger());
    const list = await screen.findByRole("listbox");
    expect(
      within(list)
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(["10", "20", "30", "50"]);
    expect(within(list).getByRole("option", { name: "30" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("writes the chosen size to the URL and goes back to page 1", async () => {
    const { router } = renderPager("?tab=live&page=3", 312);
    const user = userEvent.setup();
    expect(trigger()).toHaveTextContent("10");
    await user.click(trigger());
    await user.click(
      within(await screen.findByRole("listbox")).getByRole("option", { name: "20" }),
    );
    expect(router.state.location.search).toBe("?tab=live&size=20");
    expect(router.state.historyAction).toBe("REPLACE");
    expect(trigger()).toHaveTextContent("20");
    expect(within(nav()).getByText("1–20 trên 312 bài giao")).toBeInTheDocument();
    expect(within(nav()).getByText("Trang 1 trên 16")).toBeInTheDocument();
  });

  it("offers a screen's own sizes", async () => {
    const { router } = renderPager("", 100, { sizes: [24, 48] });
    expect(trigger()).toHaveTextContent("24");
    expect(within(nav()).getByText("1–24 trên 100 bài giao")).toBeInTheDocument();
    const user = userEvent.setup();
    await user.click(trigger());
    const list = await screen.findByRole("listbox");
    expect(
      within(list)
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(["24", "48"]);
    await user.click(within(list).getByRole("option", { name: "48" }));
    expect(router.state.location.search).toBe("?size=48");
  });

  it("is the deck's 32px control: the primitive's deck classes restated, not doubled", () => {
    renderPager("", 312);
    expect(trigger()).toHaveAttribute("data-size", "sm");
    const own = classes(trigger());
    expect(own).toEqual(
      expect.arrayContaining([
        "border-border",
        "bg-card",
        "rounded-seg",
        "text-fg",
        "font-normal",
        "px-2.5",
        "pr-2",
        "py-0",
        "in-data-[scale=deck]:bg-card",
        "dark:in-data-[scale=deck]:bg-card",
        "dark:in-data-[scale=deck]:hover:bg-card",
        "in-data-[scale=deck]:text-sm",
      ]),
    );
    for (const replaced of [
      "border-input",
      "bg-transparent",
      "rounded-md",
      "px-3",
      "py-2",
      "in-data-[scale=deck]:bg-bg",
      "dark:in-data-[scale=deck]:bg-bg",
      "dark:in-data-[scale=deck]:hover:bg-bg",
      "in-data-[scale=deck]:text-ui",
      "dark:bg-input/30",
      "dark:hover:bg-input/50",
    ]) {
      expect(own).not.toContain(replaced);
    }
  });
});

describe("below 768px", () => {
  const present = () => ({
    select: within(nav()).queryByRole("combobox") !== null,
    label: within(nav()).queryByText("Số dòng mỗi trang") !== null,
    links: within(nav())
      .getAllByRole("link")
      .map((link) => link.getAttribute("aria-label")),
  });

  it("drops rows per page, first and last, and nothing else", () => {
    viewport("phone");
    renderPager("?page=3", 312);
    expect(present()).toEqual({
      select: false,
      label: false,
      links: ["Trang trước", "Trang sau"],
    });
    expect(within(nav()).getByText("21–30 trên 312 bài giao")).toBeInTheDocument();
    expect(within(nav()).getByText("Trang 3 trên 32")).toBeInTheDocument();
    expect(arrow("previous")).toHaveAttribute("href", "/teacher/assignments?page=2");
    expect(arrow("next")).toHaveAttribute("href", "/teacher/assignments?page=4");
  });

  it("changes at 768 exactly", () => {
    const width = viewport(768);
    renderPager("?page=3", 312);
    const all = {
      select: true,
      label: true,
      links: ["Trang đầu", "Trang trước", "Trang sau", "Trang cuối"],
    };
    expect(present()).toEqual(all);
    act(() => width.resize(767));
    expect(present()).toEqual({
      select: false,
      label: false,
      links: ["Trang trước", "Trang sau"],
    });
    act(() => width.resize(800));
    expect(present()).toEqual(all);
  });
});

describe("the frame", () => {
  const CARD = ["sticky", "left-0", "border-t", "px-4", "py-2.5"];
  const PLAIN = ["px-0.5", "py-1"];

  it.each(["phone", "desktop"] as const)(
    "is the card bar by default on a %s: top border, its padding, stuck to the left edge",
    (width) => {
      viewport(width);
      renderPager("", 312);
      expect(classes(nav())).toEqual(expect.arrayContaining(CARD));
      for (const name of PLAIN) expect(classes(nav())).not.toContain(name);
    },
  );

  it.each(["phone", "desktop"] as const)(
    "is the plain bar when the screen says so on a %s: no border, not stuck",
    (width) => {
      viewport(width);
      renderPager("", 312, { frame: "plain" });
      expect(classes(nav())).toEqual(expect.arrayContaining(PLAIN));
      for (const name of CARD) expect(classes(nav())).not.toContain(name);
    },
  );

  it("names the control in both languages", async () => {
    renderPager("", 312);
    expect(nav().localName).toBe("nav");
    await english();
    expect(nav("Pagination")).toBe(document.querySelector("nav"));
    expect(screen.getByRole("combobox", { name: "Rows per page" })).toBeInTheDocument();
    for (const name of ["First page", "Previous page", "Next page", "Last page"]) {
      expect(screen.getByRole("link", { name })).toBeInTheDocument();
    }
  });
});
