import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { focusManager } from "@tanstack/react-query";
import StudentClassesPage from "@/features/classes/pages/StudentClassesPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import { viewport } from "@tests/support/viewport";
import { BASE, renderAt } from "./support";
import "@/lib/i18n";

function classes(items: unknown[], counts: "load" | "fail" = "load") {
  server.use(
    http.get(`${BASE}/app/assignments`, () =>
      counts === "fail"
        ? new Response(null, { status: 500 })
        : contractJson("/app/assignments", "get", 200, {
            dueNow: [],
            upcoming: [],
            completed: [],
          }),
    ),
    http.get(`${BASE}/app/classes`, () =>
      contractJson("/app/classes", "get", 200, { items }),
    ),
  );
  return renderAt("/app/classes", [
    { path: "/app/classes", element: <StudentClassesPage /> },
  ]);
}

beforeEach(() => viewport("phone"));
afterEach(() => {
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
});

describe("/app/classes", () => {
  it("lists the classes joined, with the way into another", async () => {
    classes([
      {
        id: "018f0000-0000-7000-8000-0000000000c1",
        name: "IELTS Foundation — Lớp tối T3/T5",
        description: "Thứ 3 và thứ 5, 19:30–21:00.",
        teacherName: "Cô Thương",
        joinedAt: "2026-07-12T01:00:00Z",
      },
    ]);
    expect(
      await screen.findByText("IELTS Foundation — Lớp tối T3/T5"),
    ).toBeInTheDocument();
    // S-10's second line: who teaches it, and since when.
    expect(screen.getByText("Cô Thương · tham gia 12/07")).toBeInTheDocument();
    expect(screen.getByText("Thứ 3 và thứ 5, 19:30–21:00.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Tham gia lớp" })).toHaveAttribute(
      "href",
      "/join",
    );
  });

  it("says when there are none", async () => {
    classes([]);
    expect(await screen.findByText("Bạn chưa tham gia lớp nào.")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Tham gia lớp" })).toHaveLength(1);
  });
});

const COUNTS_FAILED =
  "Chưa tải được số bài của các lớp. Bạn vẫn có thể mở danh sách bài.";
const ONE_CLASS = {
  id: "018f0000-0000-7000-8000-0000000000c1",
  name: "IELTS Foundation",
  description: null,
  teacherName: "Cô Thương",
  joinedAt: "2026-07-12T01:00:00Z",
};

describe("the counts' own failure", () => {
  afterEach(() => focusManager.setFocused());

  it("is reported when the counts never loaded", async () => {
    classes([ONE_CLASS], "fail");
    expect(await screen.findByText(COUNTS_FAILED)).toBeInTheDocument();
  });

  it("is not reported over counts that loaded and only failed to refresh", async () => {
    classes([ONE_CLASS]);
    await screen.findByText("IELTS Foundation");
    await waitFor(() => expect(screen.queryByText(COUNTS_FAILED)).toBeNull());

    let failed = 0;
    server.use(
      http.get(`${BASE}/app/assignments`, () => {
        failed += 1;
        return new Response(null, { status: 500 });
      }),
    );
    act(() => focusManager.setFocused(false));
    act(() => focusManager.setFocused(true));
    await waitFor(() => expect(failed).toBe(1));
    await act(() => new Promise((done) => setTimeout(done, 50)));

    expect(screen.queryByText(COUNTS_FAILED)).toBeNull();
  });
});
