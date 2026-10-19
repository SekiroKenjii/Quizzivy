import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { contentWidth } from "@tests/support/contentWidth";
import { useAuthStore } from "@/stores/auth";
import { http } from "msw";
import { adminUser } from "@tests/support/fixtures";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import {
  BASE,
  IMPACT,
  SAVED,
  servesRetentionImpact,
  servesSettings,
  signInAsAdmin,
} from "./support";
import { renderSettings } from "./render";
import "@/lib/i18n";

beforeEach(() => {
  contentWidth(1080);
  signInAsAdmin();
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

const answers = () =>
  screen.findByRole("combobox", { name: "Giữ câu trả lời của bài làm" });
const focus = () =>
  screen.getByRole("combobox", { name: "Giữ sự kiện rời khỏi bài thi" });
const anonymise = () =>
  screen.getByRole("combobox", { name: "Ẩn danh tài khoản đã tắt sau" });
const save = () => screen.getByRole("button", { name: "Lưu thay đổi" });

async function choose(
  user: ReturnType<typeof userEvent.setup>,
  box: HTMLElement,
  name: string,
) {
  await user.click(box);
  await user.click(await screen.findByRole("option", { name }));
}

async function openData() {
  const view = renderSettings("/admin/settings/data");
  await answers();
  return view;
}

describe("the Data & privacy section", () => {
  it("shows the saved retention with the deck's options and the anonymise wording", async () => {
    servesSettings();
    await openData();

    expect(await answers()).toHaveTextContent("2 năm");
    expect(focus()).toHaveTextContent("13 tháng (mặc định)");
    expect(anonymise()).toHaveTextContent("Không bao giờ");
    expect(anonymise()).toHaveAccessibleDescription(
      "Kết quả được giữ; tên và email bị xoá.",
    );

    const user = userEvent.setup({ delay: null });
    await user.click(focus());
    expect(screen.getAllByRole("option").map((option) => option.textContent)).toEqual([
      "6 tháng",
      "1 năm",
      "13 tháng (mặc định)",
      "2 năm",
    ]);
  });

  it("joins the unsaved-changes bar, and a longer retention saves without asking", async () => {
    const stored = servesSettings();
    const asked = servesRetentionImpact();
    await openData();
    const user = userEvent.setup({ delay: null });

    await choose(user, await answers(), "Mãi mãi");
    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
    await user.click(save());

    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]?.retention.keepAnswersMonths).toBeNull();
    expect(asked).toEqual([]);
  });

  it("asks what tonight would delete before a shorter retention is saved, and saves on confirm", async () => {
    const stored = servesSettings();
    const asked = servesRetentionImpact();
    await openData();
    const user = userEvent.setup({ delay: null });

    await choose(user, await answers(), "1 năm");
    await user.click(save());

    const dialog = await screen.findByRole("dialog", {
      name: "Rút ngắn thời gian lưu giữ?",
    });
    expect(asked).toEqual([{ ...SAVED.retention, keepAnswersMonths: 12 }]);
    expect(stored.puts).toEqual([]);
    expect(
      within(dialog).getByText(
        "Tối nay sẽ xoá câu trả lời của 120 lượt làm bài (3400 câu trả lời).",
      ),
    ).toBeVisible();
    expect(
      within(dialog).getByText("Tối nay sẽ xoá 56 sự kiện rời khỏi bài thi."),
    ).toBeVisible();
    expect(
      within(dialog).getByText("Tối nay sẽ ẩn danh 3 tài khoản đã tắt."),
    ).toBeVisible();

    expect(
      within(dialog).getByText(
        "1 tài khoản đã tắt được giữ nguyên vì còn sở hữu lớp hoặc nội dung.",
      ),
    ).toBeVisible();

    await user.click(within(dialog).getByRole("button", { name: "Lưu và xoá" }));
    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]?.retention.keepAnswersMonths).toBe(12);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("keeps the form as it was when the question is cancelled", async () => {
    const stored = servesSettings();
    servesRetentionImpact({ answers: null, focusEvents: 0, accounts: null });
    await openData();
    const user = userEvent.setup({ delay: null });

    await choose(user, focus(), "6 tháng");
    await user.click(save());
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Hiện chưa có gì bị xoá tối nay.")).toBeVisible();

    await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(stored.puts).toEqual([]);
    expect(focus()).toHaveTextContent("6 tháng");
    expect(save()).toBeVisible();
  });

  it("sends one save when the confirmation is pressed twice", async () => {
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const stored = servesSettings(SAVED, async (body) => {
      await gate;
      return contractJson("/admin/settings", "put", 200, {
        ...body,
        revision: body.expectedRevision + 1,
        updatedAt: "2026-10-15T09:00:00Z",
        updatedBy: adminUser.id,
      });
    });
    servesRetentionImpact();
    await openData();
    const user = userEvent.setup({ delay: null });

    await choose(user, await answers(), "1 năm");
    await user.click(save());
    const dialog = await screen.findByRole("dialog");
    await user.dblClick(within(dialog).getByRole("button", { name: "Lưu và xoá" }));
    release();

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(stored.puts).toHaveLength(1);
  });

  it("keeps an edit typed while the impact is being counted as an unsaved change", async () => {
    const stored = servesSettings();
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.post(`${BASE}/admin/settings/retention-impact`, async () => {
        await gate;
        return contractJson("/admin/settings/retention-impact", "post", 200, IMPACT);
      }),
    );
    const { router } = renderSettings("/admin/settings/data");
    await answers();
    const user = userEvent.setup({ delay: null });

    await choose(user, await answers(), "1 năm");
    await user.click(save());
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Đang lưu…" })).toBeVisible(),
    );
    await router.navigate("/admin/settings/organization");
    const name = await screen.findByRole("textbox", { name: "Tên tổ chức" });
    await user.clear(name);
    await user.type(name, "Tên mới");
    release();
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Lưu và xoá" }));

    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]?.organization.name).toBe("Quizzivy English Centre");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("textbox", { name: "Tên tổ chức" })).toHaveValue("Tên mới");
    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
  });
});
