import { afterEach, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import { DeckScale } from "@/components/ui/deck-scale";
import GroupEditorPage from "@/features/question-groups/pages/teacher/GroupEditorPage";
import { emptyGroup } from "@/features/question-groups/model";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { storedGroupBundle } from "@tests/support/storedGroupBundle";
import "@/lib/i18n";

const API = "http://localhost:8080";

afterEach(() => useAuthStore.setState({ user: null }));

it("returns focus to Student preview when the preview closes", async () => {
  useAuthStore.setState({ user: teacherUser });
  const bundle = emptyGroup("Passage");
  const id = bundle.group.id;
  server.use(
    http.get(`${API}/teacher/question-groups/${id}`, () =>
      contractJson("/teacher/question-groups/{id}", "get", 200, {
        bundle: storedGroupBundle(bundle),
        ownerSectionId: null,
        revision: 1,
        archivedAt: null,
        createdAt: "2026-09-24T00:00:00Z",
        updatedAt: "2026-09-24T00:00:00Z",
        assets: [],
        unavailableAssetIds: [],
      }),
    ),
  );
  const router = createMemoryRouter(
    [
      {
        path: "/teacher/question-bank/groups/:id",
        element: (
          <DeckScale>
            <GroupEditorPage />
          </DeckScale>
        ),
      },
    ],
    { initialEntries: [`/teacher/question-bank/groups/${id}`] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  const trigger = await screen.findByRole("button", { name: "Xem như học viên" });
  await user.click(trigger);
  expect(await screen.findByRole("dialog")).toBeInTheDocument();
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  await waitFor(() => expect(trigger).toHaveFocus());
});
