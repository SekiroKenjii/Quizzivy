import { afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import GroupEditorPage from "@/features/question-groups/pages/teacher/GroupEditorPage";
import { emptyGroup } from "@/features/question-groups/model";
import type { DraftScope } from "@/lib/drafts/store";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { storedGroupBundle } from "@tests/support/storedGroupBundle";
import "@/lib/i18n";

const API = "http://localhost:8080";
const drafts = vi.hoisted(() => ({ scope: null as DraftScope | null }));

vi.mock("@/lib/drafts/store", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/drafts/store")>()),
  openDraftScope: () => Promise.resolve(drafts.scope),
}));

afterEach(() => {
  useAuthStore.setState({ user: null });
  drafts.scope = null;
});

it("keeps focus on Keep local draft and leave while the local write is pending", async () => {
  useAuthStore.setState({ user: teacherUser });
  const bundle = emptyGroup("Passage");
  const id = bundle.group.id;
  let hold = false;
  let finishWrite = () => undefined as void;
  const write = vi.fn(() =>
    hold
      ? new Promise<void>((resolve) => {
          finishWrite = resolve;
        })
      : Promise.resolve(),
  );
  drafts.scope = {
    read: vi.fn().mockResolvedValue({
      owner: teacherUser.id,
      item: `group:${id}`,
      token: "draft",
      createdAt: Date.now(),
      expiresAt: Date.now() + 60_000,
      payload: { version: 1, revision: 1, testUpdatedAt: null, bundle },
    }),
    write,
    remove: vi.fn().mockResolvedValue(undefined),
  };
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
    http.put(
      `${API}/teacher/question-groups/${id}`,
      () => new Promise<never>(() => {}),
    ),
  );
  const router = createMemoryRouter(
    [
      {
        path: "/teacher/question-bank/groups/:id",
        element: <GroupEditorPage />,
      },
      { path: "/teacher/question-bank/groups", element: <p>Group list</p> },
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
  await user.click(await screen.findByRole("button", { name: "Khôi phục bản nháp" }));
  await user.click(await screen.findByRole("link", { name: "Nhóm câu hỏi" }));
  const keepLocal = await screen.findByRole("button", {
    name: "Giữ bản nháp trên máy và rời trang",
  });
  hold = true;
  write.mockClear();
  keepLocal.focus();
  await user.keyboard("{Enter}");

  await waitFor(() => expect(keepLocal).toHaveAttribute("aria-disabled", "true"));
  expect(keepLocal).toHaveFocus();
  expect(keepLocal).not.toBeDisabled();
  await user.keyboard("{Enter}");
  expect(write).toHaveBeenCalledTimes(1);

  finishWrite();
  expect(await screen.findByText("Group list")).toBeInTheDocument();
});
