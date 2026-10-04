import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { http, HttpResponse } from "msw";
import { DeckScale } from "@/components/ui/deck-scale";
import { AccountMenu } from "@/features/auth/AccountMenu";
import { writeThemePreference } from "@/lib/theme";
import { useAuthStore } from "@/stores/auth";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { STUDENT } from "../student/support";
import "@/lib/i18n";

const STUDENT_TRIGGER_CLOSED = `<button type="button" id="radix-_r_0_" aria-haspopup="menu" aria-expanded="false" data-state="closed" data-slot="dropdown-menu-trigger" aria-label="Tài khoản của Nguyễn Văn An" class="hover:bg-hover data-[state=open]:bg-hover inline-flex h-10 items-center rounded-[0.625rem] p-1"><span data-slot="avatar" aria-hidden="true" class="inline-grid flex-none place-content-center rounded-full font-semibold size-8 text-xs bg-brand-soft text-brand-ink">AN</span></button>`;

const STUDENT_TRIGGER_OPEN = `<button type="button" id="radix-_r_0_" aria-haspopup="menu" aria-expanded="true" data-state="open" data-slot="dropdown-menu-trigger" aria-label="Tài khoản của Nguyễn Văn An" class="hover:bg-hover data-[state=open]:bg-hover inline-flex h-10 items-center rounded-[0.625rem] p-1" aria-controls="radix-_r_1_" data-radix-popper-side="bottom" data-radix-popper-align="end"><span data-slot="avatar" aria-hidden="true" class="inline-grid flex-none place-content-center rounded-full font-semibold size-8 text-xs bg-brand-soft text-brand-ink">AN</span></button>`;

const STUDENT_MENU = `<div data-side="bottom" data-align="end" role="menu" aria-orientation="vertical" data-state="open" data-radix-menu-content="" dir="ltr" id="radix-_r_1_" aria-labelledby="radix-_r_0_" data-slot="dropdown-menu-content" data-scale="deck" class="bg-popover text-popover-foreground z-50 min-w-40 overflow-hidden rounded-md border p-1 shadow-md data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[scale=deck]:max-h-[min(60vh,var(--radix-dropdown-menu-content-available-height))] data-[scale=deck]:overflow-y-auto data-[scale=deck]:bg-card data-[scale=deck]:shadow-float data-[scale=deck]:z-(--z-popover) data-[scale=deck]:w-60 data-[scale=deck]:rounded-lg data-[scale=deck]:p-1.5" style="outline: none; --radix-dropdown-menu-content-transform-origin: var(--radix-popper-transform-origin); --radix-dropdown-menu-content-available-width: var(--radix-popper-available-width); --radix-dropdown-menu-content-available-height: var(--radix-popper-available-height); --radix-dropdown-menu-trigger-width: var(--radix-popper-anchor-width); --radix-dropdown-menu-trigger-height: var(--radix-popper-anchor-height); pointer-events: auto;" tabindex="-1" data-orientation="vertical"><div class="mb-1 flex items-center gap-2.5 border-b px-2 pt-1.5 pb-2.5"><span data-slot="avatar" aria-hidden="true" class="inline-grid flex-none place-content-center rounded-full font-semibold size-8 text-xs bg-brand-soft text-brand-ink">AN</span><div class="min-w-0"><p class="truncate text-sm font-semibold">Nguyễn Văn An</p><p class="text-muted-fg truncate text-xs">an@example.com</p></div></div><a role="menuitem" data-slot="dropdown-menu-item" data-variant="default" class="relative flex cursor-default items-center gap-2 rounded-sm px-2 py-1.5 outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[variant=destructive]:text-destructive-ink data-[variant=destructive]:focus:bg-destructive/10 [&amp;_svg]:shrink-0 [&amp;_svg:not([class*='size-'])]:size-4 in-data-[scale=deck]:data-[variant=destructive]:text-danger-ink in-data-[scale=deck]:data-[variant=destructive]:focus:bg-hover in-data-[scale=deck]:data-[variant=destructive]:focus:text-danger-ink focus:bg-hover focus:text-fg text-sm in-data-[scale=deck]:gap-2.5 in-data-[scale=deck]:p-2 in-data-[scale=deck]:leading-4 in-data-[scale=deck]:data-[disabled]:opacity-50 in-data-[scale=deck]:[&amp;_svg:not([class*='size-'])]:size-[0.9375rem]" tabindex="-1" data-orientation="vertical" data-radix-collection-item="" href="/app/settings" data-discover="true"><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-settings" aria-hidden="true"><path d="M9.671 4.136a2.34 2.34 0 0 1 4.659 0 2.34 2.34 0 0 0 3.319 1.915 2.34 2.34 0 0 1 2.33 4.033 2.34 2.34 0 0 0 0 3.831 2.34 2.34 0 0 1-2.33 4.033 2.34 2.34 0 0 0-3.319 1.915 2.34 2.34 0 0 1-4.659 0 2.34 2.34 0 0 0-3.32-1.915 2.34 2.34 0 0 1-2.33-4.033 2.34 2.34 0 0 0 0-3.831A2.34 2.34 0 0 1 6.35 6.051a2.34 2.34 0 0 0 3.319-1.915"></path><circle cx="12" cy="12" r="3"></circle></svg>Cài đặt</a><div role="menuitem" data-slot="dropdown-menu-item" data-variant="default" class="relative flex cursor-default items-center gap-2 rounded-sm px-2 py-1.5 outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[variant=destructive]:text-destructive-ink data-[variant=destructive]:focus:bg-destructive/10 [&amp;_svg]:shrink-0 [&amp;_svg:not([class*='size-'])]:size-4 in-data-[scale=deck]:data-[variant=destructive]:text-danger-ink in-data-[scale=deck]:data-[variant=destructive]:focus:bg-hover in-data-[scale=deck]:data-[variant=destructive]:focus:text-danger-ink focus:bg-hover focus:text-fg text-sm in-data-[scale=deck]:gap-2.5 in-data-[scale=deck]:p-2 in-data-[scale=deck]:leading-4 in-data-[scale=deck]:data-[disabled]:opacity-50 in-data-[scale=deck]:[&amp;_svg:not([class*='size-'])]:size-[0.9375rem]" tabindex="-1" data-orientation="vertical" data-radix-collection-item=""><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-moon" aria-hidden="true"><path d="M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401"></path></svg>Chế độ tối</div><div role="menuitem" data-slot="dropdown-menu-item" data-variant="default" class="relative flex cursor-default items-center gap-2 rounded-sm px-2 py-1.5 outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[variant=destructive]:text-destructive-ink data-[variant=destructive]:focus:bg-destructive/10 [&amp;_svg]:shrink-0 [&amp;_svg:not([class*='size-'])]:size-4 in-data-[scale=deck]:data-[variant=destructive]:text-danger-ink in-data-[scale=deck]:data-[variant=destructive]:focus:bg-hover in-data-[scale=deck]:data-[variant=destructive]:focus:text-danger-ink focus:bg-hover text-sm in-data-[scale=deck]:gap-2.5 in-data-[scale=deck]:p-2 in-data-[scale=deck]:leading-4 in-data-[scale=deck]:data-[disabled]:opacity-50 in-data-[scale=deck]:[&amp;_svg:not([class*='size-'])]:size-[0.9375rem] text-danger-ink focus:text-danger-ink border-t" tabindex="-1" data-orientation="vertical" data-radix-collection-item=""><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-log-out" aria-hidden="true"><path d="m16 17 5-5-5-5"></path><path d="M21 12H9"></path><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path></svg>Đăng xuất</div></div>`;

type Form = "expanded" | "collapsed";

function mount(element: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <DeckScale>{element}</DeckScale>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

function foot(sidebar: Form, user = teacherUser) {
  useAuthStore.setState({ user });
  return mount(<AccountMenu deck sidebar={sidebar} settingsTo="/teacher/settings" />);
}

const account = (name = "Trần Thị Bình") =>
  screen.getByRole("button", { name: `Tài khoản của ${name}` });
const items = () =>
  within(screen.getByRole("menu"))
    .getAllByRole("menuitem")
    .map((item) => item.textContent);

beforeEach(() => {
  writeThemePreference("light");
});

afterEach(() => {
  useAuthStore.getState().clearSession();
  writeThemePreference("light");
  localStorage.clear();
});

describe("the student's account menu, which the teacher's form must not move", () => {
  it("is byte for byte what the student shell drew before the sidebar form existed", async () => {
    useAuthStore.setState({ user: STUDENT });
    const user = mount(<AccountMenu settingsTo="/app/settings" deck />);
    const trigger = account("Nguyễn Văn An");
    expect(trigger.outerHTML).toBe(STUDENT_TRIGGER_CLOSED);

    await user.click(trigger);
    expect(trigger.outerHTML).toBe(STUDENT_TRIGGER_OPEN);
    expect(screen.getByRole("menu").outerHTML).toBe(STUDENT_MENU);
  });
});

describe("the account row at the foot of the teacher sidebar", () => {
  it("shows a square avatar, the name and the role of a teacher", () => {
    foot("expanded");
    const row = account();
    expect(row).toHaveClass("w-full", "gap-2.5", "rounded-md", "p-1.5", "text-left");
    expect(row).not.toHaveClass("justify-center", "h-10");
    const avatar = row.querySelector("[data-slot='avatar']");
    expect(avatar).toHaveTextContent("BT");
    expect(avatar).toHaveClass(
      "size-8",
      "rounded-md",
      "bg-brand-soft",
      "text-brand-ink",
    );
    expect(avatar).not.toHaveClass("rounded-full", "rounded-lg");
    expect(within(row).getByText("Trần Thị Bình")).toHaveClass(
      "text-sm",
      "font-medium",
    );
    expect(within(row).getByText("Giáo viên")).toHaveClass("text-muted-fg", "text-xs");
    expect(within(row).queryByText("Quản trị viên")).toBeNull();
  });

  it("names the role of a user with the admin workspace Admin", () => {
    foot("expanded", adminUser);
    const row = account("Thuong");
    expect(within(row).getByText("Quản trị viên")).toBeInTheDocument();
    expect(within(row).queryByText("Giáo viên")).toBeNull();
  });

  it("shows the avatar alone when the sidebar is collapsed, and keeps its name", () => {
    foot("collapsed");
    const row = account();
    expect(row).toHaveClass("w-full", "justify-center");
    expect(row).toHaveTextContent(/^BT$/);
    expect(row.querySelector("[data-slot='avatar']")).toHaveClass("rounded-md");
  });

  it("opens the deck's menu above the row, from its left edge", async () => {
    const user = foot("expanded");
    await user.click(account());

    const menu = screen.getByRole("menu");
    expect(menu).toHaveAttribute("data-side", "top");
    expect(menu).toHaveAttribute("data-align", "start");
    expect(menu.dataset["scale"]).toBe("deck");
    expect(menu).toHaveClass(
      "data-[scale=deck]:w-60",
      "data-[scale=deck]:rounded-lg",
      "data-[scale=deck]:p-1.5",
    );
    expect(within(menu).getByText("Trần Thị Bình")).toHaveClass("font-semibold");
    expect(within(menu).getByText("giaovien@example.com")).toBeInTheDocument();
    expect(menu.querySelector("[data-slot='avatar']")).toHaveClass("rounded-md");
    expect(menu.querySelector("[data-slot='avatar']")).not.toHaveClass("rounded-full");
  });

  it.each(["expanded", "collapsed"] as const)(
    "offers Settings, the theme and Sign out, and no Admin console, when %s",
    async (sidebar) => {
      const user = foot(sidebar, adminUser);
      await user.click(account("Thuong"));

      expect(items()).toEqual(["Cài đặt", "Chế độ tối", "Đăng xuất"]);
      expect(screen.getByRole("menuitem", { name: "Cài đặt" })).toHaveAttribute(
        "href",
        "/teacher/settings",
      );
      expect(screen.getByRole("menuitem", { name: "Đăng xuất" })).toHaveClass(
        "text-danger-ink",
        "border-t",
      );
    },
  );

  it("switches the theme from its menu", async () => {
    const user = foot("expanded");
    await user.click(account());
    await user.click(screen.getByRole("menuitem", { name: "Chế độ tối" }));
    expect(document.documentElement).toHaveClass("dark");
    expect(localStorage.getItem("quizzivy.theme")).toBe("dark");

    await user.click(account());
    expect(items()).toEqual(["Cài đặt", "Chế độ sáng", "Đăng xuất"]);
  });

  it("signs out on the server and here", async () => {
    let ended = 0;
    server.use(
      http.post("http://localhost:8080/auth/logout", () => {
        ended += 1;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const user = foot("expanded");
    await user.click(account());
    await user.click(screen.getByRole("menuitem", { name: "Đăng xuất" }));
    await waitFor(() => expect(ended).toBe(1));
    await waitFor(() => expect(useAuthStore.getState().user).toBeNull());
  });
});
