import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { onlineManager } from "@tanstack/react-query";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse } from "msw";
import { ORGANIZATION_SETTINGS_KEY } from "@/features/organization/api";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { adminUser } from "@tests/support/fixtures";
import { useAuthStore } from "@/stores/auth";
import {
  SAVED,
  failure,
  servesImpact,
  servesPublicOrganization,
  servesSettings,
  signInAsAdmin,
} from "./support";
import { renderSettings } from "./render";
import "@/lib/i18n";

const SENTENCE = "Bạn có thay đổi chưa lưu.";

beforeEach(() => {
  contentWidth(1080);
  signInAsAdmin();
  servesImpact();
});

afterEach(() => {
  onlineManager.setOnline(true);
  useAuthStore.getState().clearSession();
});

const nav = () =>
  within(screen.getByRole("navigation", { name: "Mục cài đặt hệ thống" }));
const name = () => screen.findByRole("textbox", { name: "Tên tổ chức" });
const field = (label: string) => screen.getByRole("textbox", { name: label });
const save = () => screen.getByRole("button", { name: "Lưu thay đổi" });
const discard = () => screen.getByRole("button", { name: "Bỏ thay đổi" });

async function rename(user: ReturnType<typeof userEvent.setup>, value: string) {
  const input = await name();
  await user.clear(input);
  await user.type(input, value);
}

describe("one form, one unsaved-changes bar", () => {
  it("shows no bar until something is edited, and one bar whichever section is open", async () => {
    servesSettings();
    renderSettings();
    const user = userEvent.setup();

    expect(await name()).toBeVisible();
    expect(screen.queryByText(SENTENCE)).toBeNull();

    await rename(user, "Trung tâm Quizzivy");
    expect(screen.getAllByText(SENTENCE)).toHaveLength(1);

    await user.click(nav().getByRole("link", { name: "Đăng nhập" }));
    await user.click(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    );

    expect(screen.getAllByText(SENTENCE)).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "Lưu thay đổi" })).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "Bỏ thay đổi" })).toHaveLength(1);
  });

  it("keeps an edit in the section it was made in while another is open", async () => {
    servesSettings();
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");

    await user.click(nav().getByRole("link", { name: "Đăng nhập" }));
    await user.click(nav().getByRole("link", { name: "Tổ chức" }));

    expect(field("Tên tổ chức")).toHaveValue("Trung tâm Quizzivy");
    expect(screen.getByText(SENTENCE)).toBeVisible();
  });

  it("puts every edit back, in both sections, when the changes are discarded", async () => {
    servesSettings();
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");
    await user.click(nav().getByRole("link", { name: "Đăng nhập" }));
    await user.click(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    );
    expect(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    ).not.toBeChecked();

    await user.click(discard());

    expect(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    ).toBeChecked();
    expect(
      screen.getByRole("textbox", { name: "Tên tổ chức", hidden: true }),
    ).toHaveValue("Quizzivy English Centre");
    expect(screen.queryByText(SENTENCE)).toBeNull();
  });
});

describe("Save changes", () => {
  it("sends the whole document at the revision that was read, and empty optional lines as null", async () => {
    const stored = servesSettings();
    servesPublicOrganization();
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "  Trung tâm Quizzivy  ");
    await user.clear(field("Tên viết tắt"));
    await user.clear(field("Địa chỉ"));
    await user.click(nav().getByRole("link", { name: "Đăng nhập" }));
    await user.click(
      screen.getByRole("switch", { name: "Bắt buộc đổi mật khẩu ở lần đăng nhập đầu" }),
    );

    await user.click(save());

    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]).toEqual({
      organization: {
        name: "Trung tâm Quizzivy",
        shortName: null,
        website: "quizzivy.com",
        contactEmail: "hello@quizzivy.com",
        frontDeskPhone: "028 7300 1234",
        frontDeskHours: "8:00 – 20:00 hằng ngày",
        address: null,
        logo: { light: null, dark: null },
      },
      signIn: {
        google: true,
        password: true,
        forcePasswordChange: false,
        joinCodes: true,
        allowedGoogleDomains: ["quizzivy.com", "gmail.com"],
      },
      retention: SAVED.retention,
      expectedRevision: 7,
    });
  });

  it("confirms with a toast, closes the bar and reads the front door again past the browser's cache", async () => {
    const stored = servesSettings();
    const front = servesPublicOrganization();
    const { client } = renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");

    await user.click(save());

    expect((await screen.findAllByText("Đã lưu cài đặt")).length).toBeGreaterThan(0);
    await waitFor(() => expect(screen.queryByText(SENTENCE)).toBeNull());
    expect(field("Tên tổ chức")).toHaveValue("Trung tâm Quizzivy");
    expect(client.getQueryData(ORGANIZATION_SETTINGS_KEY)).toMatchObject({
      revision: 8,
    });
    await waitFor(() => expect(front.map(({ cache }) => cache)).toContain("no-cache"));

    await rename(user, "Trung tâm Quizzivy 2");
    await user.click(save());
    await waitFor(() => expect(stored.puts).toHaveLength(2));
    expect(stored.puts[1]?.expectedRevision).toBe(8);
  });

  it("keeps what was typed while the save was in flight as an unsaved change", async () => {
    let release: () => void = () => undefined;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    const stored = servesSettings(SAVED, async ({ expectedRevision, ...rest }) => {
      await held;
      return contractJson("/admin/settings", "put", 200, {
        ...rest,
        revision: expectedRevision + 1,
        updatedAt: "2026-10-15T09:00:00Z",
        updatedBy: adminUser.id,
      });
    });
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");
    await user.click(save());
    await waitFor(() => expect(stored.puts).toHaveLength(1));

    await user.type(field("Địa chỉ"), " (tầng 3)");
    release();

    await waitFor(() =>
      expect(screen.getAllByText("Đã lưu cài đặt").length).toBeGreaterThan(0),
    );
    expect(field("Tên tổ chức")).toHaveValue("Trung tâm Quizzivy");
    expect(field("Địa chỉ")).toHaveValue("12 Nguyễn Huệ, Quận 1, TP.HCM (tầng 3)");
    expect(screen.getByText(SENTENCE)).toBeVisible();

    await user.click(save());

    await waitFor(() => expect(stored.puts).toHaveLength(2));
    expect(stored.puts[1]).toMatchObject({
      organization: {
        name: "Trung tâm Quizzivy",
        address: "12 Nguyễn Huệ, Quận 1, TP.HCM (tầng 3)",
      },
      expectedRevision: 8,
    });
    await waitFor(() => expect(screen.queryByText(SENTENCE)).toBeNull());
  });

  it("does not send a document whose name is empty, and says so on the field", async () => {
    const stored = servesSettings();
    renderSettings();
    const user = userEvent.setup();
    const input = await name();
    await user.clear(input);

    await user.click(save());

    expect(await screen.findByText("Hãy nhập tên tổ chức.")).toBeVisible();
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveFocus();
    expect(stored.puts).toHaveLength(0);
  });

  it("sends the revision the form was read at even when a newer one has been heard of since", async () => {
    const stored = servesSettings();
    const { client } = renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");
    stored.document = { ...SAVED, revision: 8, updatedAt: "2026-10-15T08:00:00Z" };

    await act(() => client.invalidateQueries({ queryKey: ORGANIZATION_SETTINGS_KEY }));
    await waitFor(() => expect(stored.reads).toBeGreaterThan(1));
    expect(field("Tên tổ chức")).toHaveValue("Trung tâm Quizzivy");
    await user.click(save());

    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]?.expectedRevision).toBe(7);
  });

  it("offers to reload when the settings were saved elsewhere, and reads them again on Reload", async () => {
    const stored = servesSettings(SAVED, () =>
      contractJson(
        "/admin/settings",
        "put",
        409,
        failure("STALE_WRITE", "Cài đặt đã được người khác thay đổi."),
      ),
    );
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");

    await user.click(save());

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Cài đặt đã được người khác thay đổi.");
    expect(discard()).toBeVisible();
    stored.document = {
      ...SAVED,
      organization: { ...SAVED.organization, name: "Tên mới từ nơi khác" },
      revision: 9,
    };
    const reads = stored.reads;

    await user.click(
      within(alert.parentElement!).getByRole("button", { name: "Tải lại" }),
    );

    await waitFor(() =>
      expect(field("Tên tổ chức")).toHaveValue("Tên mới từ nơi khác"),
    );
    expect(stored.reads).toBe(reads + 1);
    expect(screen.queryByText(SENTENCE)).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByRole("button", { name: "Tải lại" })).toBeNull();
  });

  it("shows the server's refusal of a field on that field and keeps the edits", async () => {
    servesSettings(SAVED, () =>
      contractJson(
        "/admin/settings",
        "put",
        400,
        failure("VALIDATION_FAILED", "Cài đặt không hợp lệ.", {
          "organization.website": "Website không hợp lệ.",
        }),
      ),
    );
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");

    await user.click(save());

    expect(await screen.findByText("Website không hợp lệ.")).toBeVisible();
    expect(field("Website")).toHaveAttribute("aria-invalid", "true");
    expect(field("Website")).toHaveFocus();
    expect(field("Tên tổ chức")).toHaveValue("Trung tâm Quizzivy");
    expect(discard()).toBeVisible();
  });

  it.each([
    ["the list", "signIn.allowedGoogleDomains"],
    ["one of its items", "signIn.allowedGoogleDomains.0"],
  ])("opens the section that holds a field the server refused, %s", async (_, path) => {
    servesSettings(SAVED, () =>
      contractJson(
        "/admin/settings",
        "put",
        400,
        failure("VALIDATION_FAILED", "Cài đặt không hợp lệ.", {
          [path]: "Tên miền không hợp lệ.",
        }),
      ),
    );
    const { router } = renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");

    await user.click(save());

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/admin/settings/sign-in"),
    );
    expect(await screen.findByText("Tên miền không hợp lệ.")).toBeVisible();
    await waitFor(() =>
      expect(
        screen.getByRole("textbox", { name: "Tên miền Google được phép" }),
      ).toHaveFocus(),
    );
  });

  it("says a save that failed for another reason, and leaves the edits to try again", async () => {
    servesSettings(SAVED, () => HttpResponse.error());
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");

    await user.click(save());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Đã xảy ra lỗi. Vui lòng thử lại.",
    );
    expect(screen.queryByRole("button", { name: "Tải lại" })).toBeNull();
    expect(save()).not.toHaveAttribute("aria-disabled", "true");
  });

  it("tries a save while the browser reports no connection, and says when it fails", async () => {
    const stored = servesSettings(SAVED, () => HttpResponse.error());
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");
    onlineManager.setOnline(false);

    await user.click(save());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Đã xảy ra lỗi. Vui lòng thử lại.",
    );
    expect(stored.puts).toHaveLength(1);
    expect(save()).not.toHaveAttribute("aria-disabled", "true");
  });

  it("saves from the keyboard with Enter in a text field", async () => {
    const stored = servesSettings();
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");

    await user.type(field("Tên tổ chức"), "{Enter}");

    await waitFor(() => expect(stored.puts).toHaveLength(1));
  });
});

describe("leaving with unsaved changes", () => {
  it("asks first, stays on Stay, and goes on Leave", async () => {
    servesSettings();
    const { router } = renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");

    await user.click(screen.getByRole("link", { name: "Tổng quan" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Rời Cài đặt mà không lưu?",
    });
    await user.click(within(dialog).getByRole("button", { name: "Ở lại" }));
    expect(router.state.location.pathname).toBe("/admin/settings");
    expect(field("Tên tổ chức")).toHaveValue("Trung tâm Quizzivy");

    await user.click(screen.getByRole("link", { name: "Tổng quan" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", { name: "Rời đi" }),
    );
    expect(
      await screen.findByRole("heading", { name: "Trang tổng quan" }),
    ).toBeVisible();
  });

  it("does not ask once the changes are saved", async () => {
    servesSettings();
    renderSettings();
    const user = userEvent.setup();
    await rename(user, "Trung tâm Quizzivy");
    await user.click(save());
    await waitFor(() => expect(screen.queryByText(SENTENCE)).toBeNull());

    await user.click(screen.getByRole("link", { name: "Tổng quan" }));

    expect(
      await screen.findByRole("heading", { name: "Trang tổng quan" }),
    ).toBeVisible();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
