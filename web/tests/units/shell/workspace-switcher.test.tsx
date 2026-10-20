import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { rememberWorkspace } from "@/features/auth/accountPreferences";
import { WorkspaceSwitcher } from "@/layouts/shell/WorkspaceSwitcher";
import type { StaffWorkspace } from "@/layouts/shell/nav";
import { useAuthStore } from "@/stores/auth";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import "@/lib/i18n";

vi.mock("@/features/auth/accountPreferences", () => ({ rememberWorkspace: vi.fn() }));

const adminOnly: typeof adminUser = { ...adminUser, workspaces: ["admin"] };

function mount(user: typeof teacherUser, workspace: StaffWorkspace, collapsed = false) {
  vi.spyOn(console, "warn").mockImplementation(() => {});
  useAuthStore.setState({ user });
  render(
    <MemoryRouter>
      <WorkspaceSwitcher workspace={workspace} collapsed={collapsed} />
    </MemoryRouter>,
  );
  return document.querySelector<HTMLElement>("[data-slot='workspace']")!;
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.mocked(rememberWorkspace).mockClear();
  useAuthStore.getState().clearSession();
});

describe("the workspace block of the teacher console", () => {
  it("is a label for a teacher who holds no other console", () => {
    const block = mount(teacherUser, "teacher");

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
    const block = mount(teacherUser, "teacher");
    const tile = block.querySelector("img")!.closest("[aria-hidden]")!;

    expect(tile).toHaveAttribute("aria-hidden", "true");
    expect(tile).toHaveClass("size-8", "rounded-md", "border", "bg-card");
    expect(block.querySelector("img")).toHaveAttribute("height", "18");
    expect(block.querySelector("img")).toHaveAttribute(
      "src",
      "/brand/quizzivy-mark-color.svg",
    );
    expect(console.warn).not.toHaveBeenCalledWith(expect.stringContaining("(B-05)"));
  });

  it("leaves the tile alone when collapsed, and lets it name the product", () => {
    const block = mount(teacherUser, "teacher", true);

    expect(block).toHaveClass("justify-center");
    expect(within(block).queryByText("Quizzivy")).toBeNull();
    expect(within(block).queryByText("Không gian giáo viên")).toBeNull();
    expect(within(block).getByRole("img", { name: "Quizzivy" })).toBeInTheDocument();
    expect(block.querySelector("[aria-hidden]")).toBeNull();
  });

  it("is the deck's button and menu for a user who holds both consoles", async () => {
    const block = mount(adminUser, "teacher");
    expect(block.tagName).toBe("BUTTON");
    expect(block).toHaveClass("hover:bg-hover", "w-full");
    expect(block.querySelector("svg.lucide-chevrons-up-down")).not.toBeNull();

    await userEvent.setup().click(screen.getByRole("button", { name: /Quizzivy/ }));
    const menu = screen.getByRole("menu");
    expect(menu).toHaveClass("w-60", "p-1.5");
    expect(menu).not.toHaveClass("p-1.25");
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

  it("remembers the choice when the Admin console's row is followed", async () => {
    mount(adminUser, "teacher");
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: /Quizzivy/ }));
    await user.click(screen.getByRole("menuitem", { name: "Bảng quản trị" }));

    expect(rememberWorkspace).toHaveBeenCalledExactlyOnceWith("admin");
  });
});

describe("the workspace block of the Admin console", () => {
  it("heads the console with a shield on the primary colour, not the brand mark", () => {
    const block = mount(adminOnly, "admin");

    expect(block.tagName).toBe("DIV");
    expect(within(block).getByText("Bảng quản trị")).toHaveClass(
      "text-muted-fg",
      "text-xs",
    );
    expect(block.querySelector("img")).toBeNull();
    const tile = block.querySelector("svg.lucide-shield")!.parentElement!;
    expect(tile).toHaveClass("bg-primary", "text-primary-fg", "size-8", "rounded-md");
    expect(tile).toHaveAttribute("aria-hidden", "true");
  });

  it("names the product from the tile alone when collapsed", () => {
    const block = mount(adminOnly, "admin", true);

    expect(within(block).queryByText("Bảng quản trị")).toBeNull();
    expect(within(block).getByRole("img", { name: "Quizzivy" })).toBeInTheDocument();
  });

  it("offers the teacher console to an Admin who holds it, and marks its own row", async () => {
    mount(adminUser, "admin");
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: /Quizzivy/ }));
    const rows = within(screen.getByRole("menu")).getAllByRole("menuitem");

    expect(rows.map((row) => row.textContent)).toEqual([
      "Không gian giáo viên",
      "Bảng quản trị",
    ]);
    expect(rows[0]).toHaveAttribute("href", "/teacher");
    expect(rows[1]).toHaveAttribute("aria-current", "true");

    await user.click(rows[0]!);
    expect(rememberWorkspace).toHaveBeenCalledExactlyOnceWith("teacher");
  });
});
