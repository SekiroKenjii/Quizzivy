import { afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { GroupPickerDialog } from "@/features/tests/components/GroupPickerDialog";
import { listGroups } from "@/features/question-groups/api";
import "@/lib/i18n";

vi.mock("@/features/question-groups/api", () => ({ listGroups: vi.fn() }));

afterEach(() => vi.restoreAllMocks());

it("searches once the teacher stops typing and keeps the list while it does", async () => {
  vi.mocked(listGroups).mockImplementation(
    async (params) =>
      ({
        items: [
          {
            id: "g1",
            title: `Nhóm ${params?.q ?? ""}`.trim(),
            questionCount: 3,
          },
        ],
        total: 1,
      }) as unknown as Awaited<ReturnType<typeof listGroups>>,
  );
  const user = userEvent.setup();
  render(
    <QueryClientProvider client={new QueryClient()}>
      <GroupPickerDialog
        open
        sections={[{ id: "s1", title: "Phần 1", instructions: null, questionIds: [] }]}
        onOpenChange={vi.fn()}
        onPick={vi.fn()}
      />
    </QueryClientProvider>,
  );
  await screen.findByRole("button", { name: /Nhóm/ });

  await user.type(screen.getByRole("textbox"), "reading");
  expect(screen.getByRole("button", { name: /Nhóm/ })).toBeInTheDocument();

  await waitFor(() =>
    expect(screen.getByRole("button", { name: /Nhóm reading/ })).toBeInTheDocument(),
  );
  expect(vi.mocked(listGroups).mock.calls.map(([params]) => params?.q)).toEqual([
    "",
    "reading",
  ]);
});
