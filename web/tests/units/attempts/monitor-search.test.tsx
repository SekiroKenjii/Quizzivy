import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { Monitor } from "@/features/attempts/components/Monitor";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";
import { teacherUser } from "@tests/support/fixtures";
import { contentWidth } from "@tests/support/contentWidth";
import { assignment, defaultRows, monitor } from "./fixtures";

afterEach(() => {
  cleanup();
});

function board() {
  contentWidth(1280);
  useAuthStore.getState().setSession("token", teacherUser);
  const router = createMemoryRouter([
    {
      path: "/",
      element: (
        <div data-scale="deck">
          <Monitor
            assignment={assignment()}
            data={monitor(defaultRows())}
            selectedAttempt={null}
            onOpen={() => {}}
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
  return { router, user: userEvent.setup() };
}

describe("the monitor's roster search", () => {
  it("keeps what is typed in the box, and filters the roster and the URL once the typing pauses", async () => {
    const { router, user } = board();
    const box = screen.getByRole("searchbox");

    await user.type(box, "Hoàng ");
    expect(box).toHaveValue("Hoàng ");
    expect(router.state.location.search).toBe("");
    expect(screen.getAllByText("Phạm Gia Hân").length).toBeGreaterThan(0);

    await waitFor(() => expect(router.state.location.search).toBe("?q=Ho%C3%A0ng"));
    await waitFor(() => expect(screen.queryByText("Phạm Gia Hân")).toBeNull());
    expect(screen.getAllByText("Hoàng Tiến Dũng").length).toBeGreaterThan(0);
    expect(box).toHaveValue("Hoàng ");
  });
});
