import { beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  BANK_API,
  bankPage,
  bankQuestion,
  renderBank,
} from "@tests/support/questionBank";
import "@/lib/i18n";

const A = "018f0000-0000-7000-8000-0000000000a1";
const B = "018f0000-0000-7000-8000-0000000000a2";

let tagged: { questionIds: string[]; tags: string[] } | null = null;
let deleted: string[] = [];

beforeEach(() => {
  tagged = null;
  deleted = [];
  server.use(
    http.get(`${BANK_API}/teacher/questions`, ({ request }) => {
      const q = new URL(request.url).searchParams.get("q") ?? "";
      const all = [
        bankQuestion({ id: A, prompt: "Câu một" }),
        bankQuestion({ id: B, prompt: "Câu hai" }),
      ].filter((item) => !deleted.includes(item.id));
      const items = q === "" ? all : all.filter((item) => item.prompt.includes(q));
      return contractJson("/teacher/questions", "get", 200, bankPage(items));
    }),
    http.post(`${BANK_API}/teacher/questions/tags`, async ({ request }) => {
      tagged = (await request.json()) as { questionIds: string[]; tags: string[] };
      return HttpResponse.json({ updated: tagged.questionIds.length });
    }),
    http.delete(`${BANK_API}/teacher/questions/:id`, ({ params }) => {
      if (params["id"] === B)
        return HttpResponse.json(
          {
            error: {
              code: "QUESTION_REFERENCED",
              message: "Câu hỏi đang nằm trong đề nháp.",
              requestId: "r1",
            },
          },
          { status: 409 },
        );
      deleted.push(String(params["id"]));
      return new HttpResponse(null, { status: 204 });
    }),
  );
});

describe("the bank's bulk bar", () => {
  it("counts the selection and offers Add to test, Add tag and Delete", async () => {
    const { user } = renderBank();

    await user.click(
      await screen.findByRole("checkbox", { name: "Chọn Câu một" }, { timeout: 5000 }),
    );
    expect(screen.getByText("Đã chọn 1")).toBeInTheDocument();
    await user.click(screen.getByRole("checkbox", { name: "Chọn Câu hai" }));
    expect(screen.getByText("Đã chọn 2")).toBeInTheDocument();

    expect(screen.getByRole("button", { name: "Thêm vào đề thi" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Gắn thẻ" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Xoá" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Bỏ chọn" }));
    expect(screen.queryByText(/Đã chọn/)).toBeNull();
  });

  it("keeps a selection when the search filters the row away", async () => {
    const { user } = renderBank();

    await user.click(
      await screen.findByRole("checkbox", { name: "Chọn Câu một" }, { timeout: 5000 }),
    );
    await user.type(screen.getByLabelText(/Tìm trong nội dung câu hỏi/), "hai");

    await waitFor(() =>
      expect(within(screen.getByRole("table")).queryByText("Câu một")).toBeNull(),
    );
    expect(screen.getByText("Đã chọn 1")).toBeInTheDocument();
  });

  it("tags every selected id, not just the visible ones", async () => {
    const { user } = renderBank();

    await user.click(
      await screen.findByRole("checkbox", { name: "Chọn Câu một" }, { timeout: 5000 }),
    );
    await user.click(screen.getByRole("checkbox", { name: "Chọn Câu hai" }));
    await user.click(screen.getByRole("button", { name: "Gắn thẻ" }));

    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Thẻ"), "unit-6{Enter}");
    await user.click(within(dialog).getByRole("button", { name: "Gắn thẻ" }));

    await waitFor(() => expect(tagged).not.toBeNull());
    const byName = (a: string, b: string) => a.localeCompare(b);
    expect(tagged?.questionIds.sort(byName)).toEqual([A, B].sort(byName));
    expect(tagged?.tags).toEqual(["unit-6"]);
  });

  it("applies a tag left in the dialog's input without pressing Enter", async () => {
    const { user } = renderBank();

    await user.click(
      await screen.findByRole("checkbox", { name: "Chọn Câu một" }, { timeout: 5000 }),
    );
    await user.click(screen.getByRole("button", { name: "Gắn thẻ" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Thẻ"), "unit-9");
    await user.click(within(dialog).getByRole("button", { name: "Gắn thẻ" }));

    await waitFor(() => expect(tagged).not.toBeNull());
    expect(tagged?.tags).toEqual(["unit-9"]);
  });

  it("deletes after the deck's confirmation, and names a question a draft still holds", async () => {
    const { user } = renderBank();

    await user.click(
      await screen.findByRole("checkbox", { name: "Chọn Câu một" }, { timeout: 5000 }),
    );
    await user.click(screen.getByRole("checkbox", { name: "Chọn Câu hai" }));
    await user.click(screen.getByRole("button", { name: "Xoá" }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent(
      "Các đề đã dùng những câu này vẫn giữ bản sao của chúng. Câu hỏi sẽ bị gỡ khỏi ngân hàng.",
    );
    expect(deleted).toEqual([]);
    await user.click(within(dialog).getByRole("button", { name: "Xác nhận 2 mục" }));

    await waitFor(() => expect(deleted).toEqual([A]));
    expect(
      await within(dialog).findByText("Câu hai: Câu hỏi đang nằm trong đề nháp."),
    ).toBeInTheDocument();
    expect(screen.getByText("Đã chọn 1")).toBeInTheDocument();
  });
});
