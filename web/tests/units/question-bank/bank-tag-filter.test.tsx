import { beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  BANK_API,
  bankPage,
  bankQuestion,
  renderBank,
} from "@tests/support/questionBank";
import "@/lib/i18n";

let requests: URL[] = [];

beforeEach(() => {
  requests = [];
  server.use(
    http.get(`${BANK_API}/teacher/questions`, ({ request }) => {
      requests.push(new URL(request.url));
      return contractJson(
        "/teacher/questions",
        "get",
        200,
        bankPage([bankQuestion()], {
          tags: ["grammar", "listening", "nghe hiểu", "unit 5"],
        }),
      );
    }),
  );
});

async function tagBox() {
  const filters = await screen.findByRole("complementary", { name: "Bộ lọc" });
  return within(filters).getByRole("combobox", { name: "Thẻ" });
}

describe("the bank's tag filter", () => {
  it("lists the server's tags, accent-insensitively, and Enter takes the first match", async () => {
    const { user, router } = renderBank();
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    const box = await tagBox();

    await user.click(box);
    const list = screen.getByRole("listbox", { name: "Thẻ" });
    expect(
      within(list)
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(["grammar", "listening", "nghe hiểu", "unit 5"]);

    await user.type(box, "nghe");
    expect(within(list).getAllByRole("option")).toHaveLength(1);
    await user.keyboard("{Enter}");

    expect(box).toHaveValue("");
    await waitFor(() =>
      expect(new URLSearchParams(router.state.location.search).getAll("tag")).toEqual([
        "nghe hiểu",
      ]),
    );
    expect(within(list).getByRole("option", { name: "nghe hiểu" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("moves through the list with the arrow keys and closes on Esc", async () => {
    const { user, router } = renderBank();
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    const box = await tagBox();

    await user.click(box);
    await user.keyboard("{ArrowDown}{ArrowDown}");
    const active = box.getAttribute("aria-activedescendant");
    expect(document.getElementById(active ?? "")).toHaveTextContent("listening");
    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(new URLSearchParams(router.state.location.search).getAll("tag")).toEqual([
        "listening",
      ]),
    );

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(box).toHaveAttribute("aria-expanded", "false");
  });

  it("says when no tag matches", async () => {
    const { user } = renderBank();
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    await user.type(await tagBox(), "zzz");
    expect(screen.getByText("Không có thẻ nào khớp “zzz”")).toBeInTheDocument();
  });

  it("shows the chosen tags as chips that remove themselves, with Clear", async () => {
    const { user, router } = renderBank(
      "/teacher/question-bank?tag=grammar&tag=unit%205",
    );
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    const filters = screen.getByRole("complementary", { name: "Bộ lọc" });

    await user.click(within(filters).getByRole("button", { name: "Bỏ thẻ grammar" }));
    await waitFor(() =>
      expect(new URLSearchParams(router.state.location.search).getAll("tag")).toEqual([
        "unit 5",
      ]),
    );
    await user.click(within(filters).getByRole("button", { name: "Xoá" }));
    await waitFor(() => expect(router.state.location.search).toBe(""));
  });

  it("offers Match Any | All from two tags, and sends All only with two", async () => {
    const { user, router } = renderBank("/teacher/question-bank?tag=grammar");
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    const filters = screen.getByRole("complementary", { name: "Bộ lọc" });
    expect(within(filters).queryByRole("group", { name: "Cách khớp thẻ" })).toBeNull();

    const box = await tagBox();
    await user.type(box, "unit{Enter}");
    const match = await within(filters).findByRole("group", { name: "Cách khớp thẻ" });
    await user.click(within(match).getByRole("button", { name: "Tất cả" }));

    await waitFor(() => {
      expect(new URLSearchParams(router.state.location.search).get("tagMatch")).toBe(
        "all",
      );
      expect(requests.at(-1)?.searchParams.get("tagMatch")).toBe("all");
    });
  });
});
