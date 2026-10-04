import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { Shield } from "lucide-react";
import { DeckScale } from "@/components/ui/deck-scale";
import {
  WorkspaceSwitcher,
  type WorkspaceDestination,
} from "@/layouts/shell/WorkspaceSwitcher";
import { useAuthStore } from "@/stores/auth";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const ADMIN_CONSOLE: WorkspaceDestination[] = [
  { id: "admin", label: "Bảng quản trị", to: "/admin", icon: Shield },
];

function mount(
  user: typeof teacherUser,
  destinations: readonly WorkspaceDestination[],
  collapsed = false,
) {
  vi.spyOn(console, "warn").mockImplementation(() => {});
  useAuthStore.setState({ user });
  render(
    <MemoryRouter>
      <DeckScale>
        <WorkspaceSwitcher collapsed={collapsed} destinations={destinations} />
      </DeckScale>
    </MemoryRouter>,
  );
  return document.querySelector<HTMLElement>("[data-slot='workspace']")!;
}

afterEach(() => {
  vi.restoreAllMocks();
  useAuthStore.getState().clearSession();
});

describe("the workspace block", () => {
  it.each([
    ["a teacher", teacherUser],
    ["an admin", adminUser],
  ])("is a label for %s while there is no other console to open", (_who, user) => {
    const block = mount(user, []);

    expect(block.tagName).toBe("DIV");
    expect(screen.queryByRole("button")).toBeNull();
    expect(within(block).getByText("Quizzivy")).toHaveClass("text-ui", "font-semibold");
    expect(within(block).getByText("Không gian giáo viên")).toHaveClass(
      "text-muted-fg",
      "text-xs",
    );
    expect(block.querySelector("svg")).toBeNull();
  });

  it("draws the mark 18px high in its tile, hidden while the name is written beside it", () => {
    const block = mount(teacherUser, []);
    const tile = block.querySelector("img")!.closest("[aria-hidden]")!;

    expect(tile).toHaveAttribute("aria-hidden", "true");
    expect(tile).toHaveClass("size-8", "rounded-md", "border", "bg-card");
    expect(block.querySelector("img")).toHaveAttribute("height", "18");
    expect(block.querySelector("img")).toHaveAttribute(
      "src",
      "/brand/quizzivy-mark-color.svg",
    );
  });

  it("leaves the tile alone when collapsed, and lets it name the product", () => {
    const block = mount(teacherUser, [], true);

    expect(block).toHaveClass("justify-center");
    expect(within(block).queryByText("Quizzivy")).toBeNull();
    expect(within(block).queryByText("Không gian giáo viên")).toBeNull();
    expect(within(block).getByRole("img", { name: "Quizzivy" })).toBeInTheDocument();
    expect(block.querySelector("[aria-hidden]")).toBeNull();
  });

  it("stays a label for a teacher even when a destination is passed", () => {
    const block = mount(teacherUser, ADMIN_CONSOLE);
    expect(block.tagName).toBe("DIV");
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("is the deck's button and menu for an admin with a console to open", async () => {
    const block = mount(adminUser, ADMIN_CONSOLE);
    expect(block.tagName).toBe("BUTTON");
    expect(block).toHaveClass("hover:bg-hover", "w-full");
    expect(block.querySelector("svg.lucide-chevrons-up-down")).not.toBeNull();

    await userEvent.setup().click(screen.getByRole("button", { name: /Quizzivy/ }));
    const menu = screen.getByRole("menu");
    expect(menu.dataset["scale"]).toBe("deck");
    expect(menu).toHaveClass("data-[scale=deck]:w-60", "data-[scale=deck]:p-1.5");
    expect(within(menu).getByText("Chuyển không gian làm việc")).toBeInTheDocument();
    const rows = within(menu).getAllByRole("menuitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      "Không gian giáo viên",
      "Bảng quản trị",
    ]);
    expect(rows[0]).toHaveAttribute("aria-current", "true");
    expect(rows[0]).toHaveClass("bg-muted", "font-medium");
    expect(rows[1]).toHaveAttribute("href", "/admin");
    expect(rows[1]).not.toHaveAttribute("aria-current");
  });
});
