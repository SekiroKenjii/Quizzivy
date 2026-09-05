import { afterEach, describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { http } from "msw";
import StudentClassesPage from "@/features/classes/pages/StudentClassesPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import { BASE, renderAt } from "./support";
import "@/lib/i18n";

function classes(items: unknown[]) {
  server.use(
    http.get(`${BASE}/app/classes`, () =>
      contractJson("/app/classes", "get", 200, { items }),
    ),
  );
  return renderAt("/app/classes", [
    { path: "/app/classes", element: <StudentClassesPage /> },
  ]);
}

afterEach(() => useAuthStore.getState().clearSession());

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
    expect(screen.getAllByRole("link", { name: "Tham gia lớp" })).toHaveLength(2);
  });
});
