import { describe, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { listStudents, resetStudentsPasswords } from "@/features/students/api";
import { server } from "@tests/support/server";

const BASE = "http://localhost:8080";

const emptyList = {
  items: [],
  facets: { total: 0, activeLast7Days: 0 },
  page: 1,
  pageSize: 50,
  total: 0,
};

function captureSearch(): { current: URLSearchParams | null } {
  const seen: { current: URLSearchParams | null } = { current: null };
  server.use(
    http.get(`${BASE}/teacher/students`, ({ request }) => {
      seen.current = new URL(request.url).searchParams;
      return HttpResponse.json(emptyList);
    }),
  );
  return seen;
}

describe("listStudents filters", () => {
  it("repeats classId once per class and sends the password flag as given", async () => {
    const seen = captureSearch();
    await listStudents({ classId: ["class-a", "class-b"], mustChangePassword: false });
    expect(seen.current?.getAll("classId")).toEqual(["class-a", "class-b"]);
    expect(seen.current?.get("mustChangePassword")).toBe("false");
  });

  it("sends neither filter when none is chosen", async () => {
    const seen = captureSearch();
    await listStudents({ classId: [] });
    expect(seen.current?.has("classId")).toBe(false);
    expect(seen.current?.has("mustChangePassword")).toBe(false);
  });
});

describe("resetStudentsPasswords", () => {
  it("posts the ids once and returns what the server reset and what it could not", async () => {
    let body: unknown = null;
    server.use(
      http.post(`${BASE}/teacher/students/reset-passwords`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(
          {
            items: [
              {
                studentId: "student-1",
                fullName: "Học viên một",
                email: "mot@example.com",
                temporaryPassword: "lop-hoc-42",
              },
            ],
            failed: [{ studentId: "student-2", code: "NOT_FOUND" }],
          },
          { headers: { "Cache-Control": "no-store" } },
        );
      }),
    );

    const result = await resetStudentsPasswords(["student-1", "student-2"]);

    expect(body).toEqual({ studentIds: ["student-1", "student-2"] });
    expect(result.items.map((item) => item.temporaryPassword)).toEqual(["lop-hoc-42"]);
    expect(result.failed).toEqual([{ studentId: "student-2", code: "NOT_FOUND" }]);
  });
});
