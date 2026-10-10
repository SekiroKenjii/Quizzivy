import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { Monitor } from "@/features/attempts/components/Monitor";
import { useAuthStore } from "@/stores/auth";
import i18n from "@/lib/i18n";
import { teacherUser } from "@tests/support/fixtures";
import { contentWidth } from "@tests/support/contentWidth";
import { assignment, defaultRows, monitor } from "./fixtures";

afterEach(() => {
  cleanup();
  void i18n.changeLanguage("vi");
});

function board(focusLossCount: number | null | undefined) {
  contentWidth(332);
  useAuthStore.getState().setSession("token", teacherUser);
  const row = { ...defaultRows()[1]!, flagged: true };
  if (focusLossCount === undefined) delete row.focusLossCount;
  else row.focusLossCount = focusLossCount;
  const onOpen = vi.fn();
  const router = createMemoryRouter([
    {
      path: "/",
      element: (
        <div data-scale="deck">
          <Monitor
            assignment={assignment()}
            data={monitor([row])}
            selectedAttempt={null}
            onOpen={onOpen}
            onRefresh={async () => {}}
          />
        </div>
      ),
    },
  ]);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { row, onOpen };
}

describe("phone roster flag and focus information", () => {
  for (const locale of ["vi", "en"]) {
    it(`keeps the actual count compact and its full ${locale} sentence accessible`, async () => {
      await i18n.changeLanguage(locale);
      const { row } = board(3);
      const label = `${i18n.t("status.attention.flagged")} · ${i18n.t("assignmentDetail.focusCount", { count: 3 })}`;
      const flag = screen.getByLabelText(label);
      expect(flag).toHaveTextContent(/^3×$/);
      expect(flag.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
      expect(flag).toHaveClass("shrink-0", "whitespace-nowrap");
      expect(
        screen.getByRole("button", { name: new RegExp(row.fullName) }),
      ).toHaveAccessibleName(
        new RegExp(i18n.t("assignmentDetail.focusCount", { count: 3 })),
      );
      expect(
        within(screen.getByRole("table")).getByText(
          i18n.t("assignmentDetail.needsGrading"),
        ),
      ).toHaveClass("truncate");
    });
  }
  it.each([undefined, null])(
    "preserves unknown flagged focus count %s without reporting zero",
    (count) => {
      board(count);
      const flag = screen.getByLabelText(`${i18n.t("status.attention.flagged")} · —`);
      expect(flag).toHaveTextContent(/^—$/);
      expect(
        screen.queryByText(i18n.t("assignmentDetail.focusCount", { count: 0 })),
      ).toBeNull();
    },
  );
});

describe("the flag marker where the Focus lost column is hidden", () => {
  it("names a flagged row from 560 to 700 and leaves an unflagged one unmarked", () => {
    contentWidth(650);
    useAuthStore.getState().setSession("token", teacherUser);
    const [first, second] = defaultRows();
    const flagged = { ...first!, flagged: true, focusLossCount: 4 };
    const calm = { ...second!, flagged: false, focusLossCount: 1 };
    const router = createMemoryRouter([
      {
        path: "/",
        element: (
          <div data-scale="deck">
            <Monitor
              assignment={assignment()}
              data={monitor([flagged, calm])}
              selectedAttempt={null}
              onOpen={vi.fn()}
              onRefresh={async () => {}}
            />
          </div>
        ),
      },
    ]);
    render(
      <QueryClientProvider client={new QueryClient()}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    const table = screen.getByRole("table");
    expect(
      within(table).getByRole("columnheader", { name: i18n.t("monitor.state") }),
    ).toBeInTheDocument();
    expect(
      within(table).queryByRole("columnheader", {
        name: i18n.t("assignmentDetail.focusLost"),
      }),
    ).toBeNull();
    const label = `${i18n.t("status.attention.flagged")} · ${i18n.t("assignmentDetail.focusCount", { count: 4 })}`;
    const marker = within(
      screen.getByRole("row", { name: new RegExp(flagged.fullName) }),
    ).getByRole("img", { name: label });
    expect(marker).toHaveTextContent(/^4×$/);
    const flaggedRow = screen.getByRole("row", { name: new RegExp(flagged.fullName) });
    expect(flaggedRow).toHaveClass("bg-danger-soft", "hover:bg-danger-soft");
    expect(flaggedRow).not.toHaveClass("hover:bg-muted");
    expect(
      within(screen.getByRole("row", { name: new RegExp(calm.fullName) })).queryByRole(
        "img",
        { name: new RegExp(i18n.t("status.attention.flagged")) },
      ),
    ).toBeNull();
  });
});
