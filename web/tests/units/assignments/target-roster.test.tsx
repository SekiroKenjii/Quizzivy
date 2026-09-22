import { beforeEach, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useTargetRoster } from "@/features/assignments/useTargetRoster";
import { fetchMembers } from "@/features/classes/api";
import { getStudent } from "@/features/students/api";

vi.mock("@/features/classes/api", () => ({ fetchMembers: vi.fn() }));
vi.mock("@/features/students/api", () => ({ getStudent: vi.fn() }));
const stats = { submittedCount: 0, flaggedCount: 0, activity: { live: false } };
const joinedAt = "2026-09-01T00:00:00Z";
function member(id: string) {
  return {
    userId: id,
    fullName: id,
    email: `${id}@example.com`,
    joinedVia: "admin" as const,
    joinedAt,
    stats,
  };
}
beforeEach(() => vi.clearAllMocks());

it("unions all membership pages and individual selections, excluding disabled students", async () => {
  vi.mocked(fetchMembers).mockImplementation(async (id, params) => ({
    items:
      id === "class-a" ? [member(params?.page === 2 ? "Bình" : "An")] : [member("An")],
    total: id === "class-a" ? 2 : 1,
    page: params?.page ?? 1,
    pageSize: 1,
  }));
  vi.mocked(getStudent).mockImplementation(async (id) => ({
    id,
    fullName: id,
    email: `${id}@example.com`,
    hasPassword: true,
    linkedProviders: [],
    mustChangePassword: false,
    createdAt: joinedAt,
    disabledAt: id === "disabled" ? joinedAt : null,
    classes: [],
    stats,
  }));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const { result } = renderHook(
    () => useTargetRoster(["class-a", "class-b"], ["An", "Chi", "disabled"]),
    {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    },
  );
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.data).toEqual({ total: 3, overlaps: ["An"] });
  expect(fetchMembers).toHaveBeenCalledTimes(3);
  client.clear();
});
