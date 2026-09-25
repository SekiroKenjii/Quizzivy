import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/TestBuilderPage";
import { emptyGroup, newGroupQuestion } from "@/features/question-groups/model";
import type { StoredGroup } from "@/features/question-groups/api";
import type { Test } from "@/features/tests/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import i18n from "@/lib/i18n";

test("a publication finding opens the owned member editor without trying the standalone bank endpoint", async () => {
  const sectionId = crypto.randomUUID();
  const question = newGroupQuestion(i18n.t);
  question.input.prompt = "Câu trong nhóm";
  const bundle = emptyGroup("Bài đọc chung");
  bundle.group.members = [{ questionId: question.id, optionOrder: "shuffle" }];
  bundle.questions = [question];
  const stamp = "2026-09-24T00:00:00Z";
  const group: StoredGroup = {
    bundle,
    ownerSectionId: sectionId,
    revision: 1,
    testUpdatedAt: stamp,
    archivedAt: null,
    createdAt: stamp,
    updatedAt: stamp,
    assets: [],
    unavailableAssetIds: [],
  };
  const draft: Test = {
    id: crypto.randomUUID(),
    title: "Đề thử nhóm",
    description: null,
    status: "draft",
    currentVersion: 0,
    totalPoints: 1,
    questionCount: 1,
    audioCount: 0,
    createdAt: stamp,
    updatedAt: stamp,
    sections: [
      {
        id: sectionId,
        ordinal: 0,
        title: "Đọc hiểu",
        instructions: null,
        questionIds: [],
        units: [{ kind: "group", id: bundle.group.id }],
      },
    ],
  };
  let standaloneReads = 0;
  server.use(
    http.get("http://localhost:8080/admin/tests/:id", () =>
      contractJson("/admin/tests/{id}", "get", 200, draft),
    ),
    http.get("http://localhost:8080/admin/question-groups/:id", () =>
      contractJson("/admin/question-groups/{id}", "get", 200, group),
    ),
    http.get("http://localhost:8080/admin/questions/:id", () => {
      standaloneReads++;
      return new Response(null, { status: 404 });
    }),
    http.post("http://localhost:8080/admin/tests/:id/publish", () =>
      Response.json(
        {
          error: { code: "PUBLISH_VALIDATION_FAILED", message: "Cần sửa nội dung" },
          violations: [
            {
              rule: "choice_has_correct_option",
              questionId: question.id,
              sectionId,
              message: "Chọn đáp án đúng",
            },
          ],
        },
        { status: 409 },
      ),
    ),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [{ path: "/admin/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/admin/tests/${draft.id}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  await screen.findByLabelText("Tên nhóm câu hỏi", { exact: true });
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Phát hành" }));
  const dialog = await screen.findByRole("dialog");
  expect(within(dialog).getByText("Câu 1 · Đọc hiểu")).toBeVisible();
  await user.click(within(dialog).getByRole("button", { name: "Đi tới" }));
  expect(await screen.findByRole("textbox", { name: "Nội dung câu hỏi" })).toHaveValue(
    "Câu trong nhóm",
  );
  expect(standaloneReads).toBe(0);
});
