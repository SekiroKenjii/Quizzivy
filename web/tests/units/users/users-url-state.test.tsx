import { afterEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import { SEARCH_MAX } from "@/features/users/api";
import i18n from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import { http } from "msw";
import { server } from "@tests/support/server";
import { BASE, renderUsers, rowOf, serveUsers, table, user } from "./support";

const NO_ONE = "Không có người dùng phù hợp.";
const NGUYEN = user("4", "Nguyễn Văn An");
const THUONG = user("1", "Thường Võ");
const FEW = [THUONG, NGUYEN];

function crowd(count: number) {
  return Array.from({ length: count }, (_, index) =>
    user(String(index + 1), `Bạn ${index + 1}`),
  );
}

afterEach(async () => {
  vi.unstubAllGlobals();
  useAuthStore.getState().clearSession();
  await act(() => i18n.changeLanguage("vi"));
});

describe("the search box and the URL", () => {
  it("keeps what is typed out of the URL and the request until the typing pauses", async () => {
    const asked = serveUsers(FEW);
    const { user: person, router } = renderUsers();
    await table();
    const before = asked.length;
    const box = screen.getByRole("searchbox");

    await person.type(box, "ngu");
    expect(box).toHaveValue("ngu");
    expect(router.state.location.search).toBe("");
    expect(asked).toHaveLength(before);

    await waitFor(() => expect(router.state.location.search).toBe("?q=ngu"));
    await waitFor(() => expect(asked.at(-1)?.get("q")).toBe("ngu"));
  });

  it("never takes a keystroke back: text typed after an applied search stays in the box", async () => {
    serveUsers(FEW);
    const { user: person, router } = renderUsers();
    await table();
    const box = screen.getByRole("searchbox");

    await person.type(box, "ngu");
    await waitFor(() => expect(router.state.location.search).toBe("?q=ngu"));
    await person.type(box, "yen");
    expect(box).toHaveValue("nguyen");
    expect(router.state.location.search).toBe("?q=ngu");

    await waitFor(() => expect(router.state.location.search).toBe("?q=nguyen"));
    expect(box).toHaveValue("nguyen");
  });

  it("leaves the box as typed when it writes the text as the server reads it", async () => {
    serveUsers(FEW);
    const { user: person, router } = renderUsers();
    await table();
    const box = screen.getByRole("searchbox");

    await person.type(box, "  ngu ");
    await waitFor(() => expect(router.state.location.search).toBe("?q=ngu"));
    expect(box).toHaveValue("  ngu ");
  });

  it("writes one character, or two, nowhere, so the page stays where it was", async () => {
    const asked = serveUsers(crowd(25));
    const { user: person, router } = renderUsers("/admin/users?page=2");
    await table();
    const box = screen.getByRole("searchbox");

    await person.type(box, "ba");
    await new Promise((resolve) => setTimeout(resolve, 450));
    expect(router.state.location.search).toBe("?page=2");
    expect(asked.at(-1)?.get("page")).toBe("2");
    expect(asked.every((query) => !query.has("q"))).toBe(true);
  });

  it("sends the page back to the first when a search is applied", async () => {
    const asked = serveUsers(crowd(25));
    const { user: person, router } = renderUsers("/admin/users?page=2");
    await table();

    await person.type(screen.getByRole("searchbox"), "ban");
    await waitFor(() => expect(router.state.location.search).toBe("?q=ban"));
    await waitFor(() => expect(asked.at(-1)?.get("page")).toBeNull());
  });

  it("keeps the search when a tab is chosen during the pause, and the tab when the search lands", async () => {
    const asked = serveUsers(FEW);
    const { user: person, router } = renderUsers();
    await table();

    await person.type(screen.getByRole("searchbox"), "ngu");
    await person.click(screen.getByRole("button", { name: /^Học viên/ }));
    expect(router.state.location.search).toBe("?tab=students");

    await waitFor(() =>
      expect(router.state.location.search).toBe("?tab=students&q=ngu"),
    );
    await waitFor(() => {
      expect(asked.at(-1)?.get("tab")).toBe("students");
      expect(asked.at(-1)?.get("q")).toBe("ngu");
    });
  });

  it("puts the text of an outside change of the URL in the box, and does so for Back", async () => {
    const asked = serveUsers(FEW);
    const { router } = renderUsers("/admin/users?q=nguyen");
    await table();
    const box = screen.getByRole("searchbox");
    expect(box).toHaveValue("nguyen");

    await act(() => router.navigate("/admin/users?q=thuong"));
    await waitFor(() => expect(box).toHaveValue("thuong"));
    await waitFor(() => expect(asked.at(-1)?.get("q")).toBe("thuong"));

    await act(() => router.navigate(-1));
    await waitFor(() => expect(box).toHaveValue("nguyen"));
    await waitFor(() => expect(asked.at(-1)?.get("q")).toBe("nguyen"));
  });

  it("empties the box when the search leaves the URL from outside", async () => {
    serveUsers(FEW);
    const { router } = renderUsers("/admin/users?q=nguyen");
    await table();
    const box = screen.getByRole("searchbox");

    await act(() => router.navigate("/admin/users"));
    await waitFor(() => expect(box).toHaveValue(""));
  });

  it("does not take the box's text for a search once the URL was changed to another", async () => {
    const asked = serveUsers(FEW);
    const { user: person, router } = renderUsers();
    await table();
    const box = screen.getByRole("searchbox");
    await person.type(box, "ngu");

    await act(() => router.navigate("/admin/users?q=thuong"));
    await waitFor(() => expect(box).toHaveValue("thuong"));
    await new Promise((resolve) => setTimeout(resolve, 450));
    expect(router.state.location.search).toBe("?q=thuong");
    expect(asked.at(-1)?.get("q")).toBe("thuong");
  });

  it("asks for no more of a search than the contract takes, and writes that to the URL", async () => {
    const asked = serveUsers(FEW);
    const { router } = renderUsers(`/admin/users?q=${"a".repeat(SEARCH_MAX + 1)}`);
    expect(await screen.findByText(NO_ONE)).toBeVisible();
    expect(asked.every((query) => (query.get("q") ?? "").length <= SEARCH_MAX)).toBe(
      true,
    );
    expect(asked.at(-1)?.get("q")).toBe("a".repeat(SEARCH_MAX));
    await waitFor(() =>
      expect(router.state.location.search).toBe(`?q=${"a".repeat(SEARCH_MAX)}`),
    );
  });

  it("shows no more of a long q from a link than is searched (QC-UL-12)", async () => {
    serveUsers(FEW);
    renderUsers(`/admin/users?q=${"a".repeat(5000)}`);
    await table();
    expect(screen.getByRole("searchbox")).toHaveValue("a".repeat(SEARCH_MAX));
  });

  it("counts the box's characters as the contract does, not in UTF-16 units", async () => {
    serveUsers(FEW);
    const { user: actor } = renderUsers();
    await table();
    const box = screen.getByRole("searchbox");
    await actor.click(box);
    await actor.paste("😀".repeat(250));
    expect([...(box as HTMLInputElement).value]).toHaveLength(SEARCH_MAX);
  });
});

describe("the page", () => {
  it("asks for no page past the highest the contract takes, and goes to the last one", async () => {
    const asked = serveUsers(FEW);
    const { router } = renderUsers("/admin/users?page=1000001");
    expect(await rowOf("Thường Võ")).toBeVisible();
    expect(asked[0]?.get("page")).toBe("1000000");
    expect(asked.every((query) => Number(query.get("page") ?? 1) <= 1_000_000)).toBe(
      true,
    );
    expect(router.state.location.search).toBe("");
  });

  it("goes to the last page when the page is past the end, without ever showing the list as empty", async () => {
    const asked = serveUsers(crowd(25));
    const seen: boolean[] = [];
    const watch = new MutationObserver(() => {
      seen.push(screen.queryByText(NO_ONE) !== null);
    });
    watch.observe(document.body, {
      childList: true,
      subtree: true,
      characterData: true,
    });
    try {
      const { router } = renderUsers("/admin/users?page=9");
      expect(await rowOf("Bạn 25")).toBeVisible();
      expect(asked[0]?.get("page")).toBe("9");
      expect(asked.at(-1)?.get("page")).toBe("3");
      expect(router.state.location.search).toBe("?page=3");
      expect(await screen.findByText("21–25 trên 25 người dùng")).toBeVisible();
    } finally {
      watch.disconnect();
    }
    expect(seen).not.toContain(true);
  });

  it.each([
    ["a tab", "tab=students"],
    ["a size", "size=20"],
    ["a search", "q=ban"],
  ])(
    "goes back to the first page when %s is changed from outside",
    async (_, change) => {
      const asked = serveUsers(crowd(25));
      const { router } = renderUsers("/admin/users?page=2");
      await table();
      expect(asked.at(-1)?.get("page")).toBe("2");

      await act(() => router.navigate(`/admin/users?page=2&${change}`));
      await waitFor(() => expect(router.state.location.search).toBe(`?${change}`));
      await waitFor(() => expect(asked.at(-1)?.get("page")).toBeNull());
    },
  );

  it("keeps the page when only the page is changed from outside", async () => {
    const asked = serveUsers(crowd(25));
    const { router } = renderUsers("/admin/users?page=2");
    await table();

    await act(() => router.navigate("/admin/users?page=3"));
    await waitFor(() => expect(asked.at(-1)?.get("page")).toBe("3"));
    expect(router.state.location.search).toBe("?page=3");
    expect(await rowOf("Bạn 25")).toBeVisible();
  });

  it("keeps a page of an empty list on the empty state (QC-UL-11)", async () => {
    const asked = serveUsers(FEW);
    const { router } = renderUsers("/admin/users?q=zzz&page=5");
    expect(await screen.findByText(NO_ONE)).toBeVisible();
    expect(asked.map((query) => query.get("page"))).toEqual(["5"]);
    expect(router.state.location.search).toBe("?q=zzz&page=5");
  });

  it("keeps the table on screen while the next page loads (QC-UL-11)", async () => {
    serveUsers(crowd(25));
    let release: () => void = () => {};
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${BASE}/admin/users`, async ({ request }) => {
        if (new URL(request.url).searchParams.get("page") === "2") await gate;
      }),
    );
    const { user: actor } = renderUsers();
    expect(await rowOf("Bạn 3")).toBeVisible();
    await actor.click(screen.getByRole("link", { name: "Trang sau" }));
    expect(screen.getByRole("table", { name: "Người dùng" })).toBeVisible();
    expect(screen.getByText("Bạn 3")).toBeVisible();
    act(() => release());
    expect(await rowOf("Bạn 13")).toBeVisible();
  });
});
