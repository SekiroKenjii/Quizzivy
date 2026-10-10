import { describe, expect, it } from "vitest";
import { http, HttpResponse } from "msw";
import { fetchClasses } from "@/features/classes/api";
import { server } from "@tests/support/server";

const BASE = "http://localhost:8080";

const emptyList = {
  items: [],
  facets: { all: 0, joinable: 0, archived: 0, students: 0 },
  page: 1,
  pageSize: 20,
  total: 0,
};

function captureSearch(): { current: URLSearchParams | null } {
  const seen: { current: URLSearchParams | null } = { current: null };
  server.use(
    http.get(`${BASE}/teacher/classes`, ({ request }) => {
      seen.current = new URL(request.url).searchParams;
      return HttpResponse.json(emptyList, { headers: { "Cache-Control": "no-store" } });
    }),
  );
  return seen;
}

describe("fetchClasses codes", () => {
  it("asks for codes only when told to", async () => {
    const asked = captureSearch();
    await fetchClasses({ withCodes: true, status: "all" });
    expect(asked.current?.get("withCodes")).toBe("true");
    expect(asked.current?.get("status")).toBe("all");
  });

  it("does not ask for codes for a picker", async () => {
    const picker = captureSearch();
    await fetchClasses({ limit: 100 });
    expect(picker.current?.has("withCodes")).toBe(false);
    await fetchClasses({ withCodes: false });
    expect(picker.current?.has("withCodes")).toBe(false);
  });
});
