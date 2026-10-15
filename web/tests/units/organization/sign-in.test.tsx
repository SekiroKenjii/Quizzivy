import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { useAuthStore } from "@/stores/auth";
import {
  NO_IMPACT,
  SAVED,
  failure,
  servesImpact,
  servesSettings,
  signInAsAdmin,
} from "./support";
import { renderSettings } from "./render";
import "@/lib/i18n";

const SENTENCE = "Bạn có thay đổi chưa lưu.";
const NO_METHOD =
  "Phải bật ít nhất một cách đăng nhập: Google, hoặc email và mật khẩu.";
const LOCKED_YOU =
  "Với chính sách này, chính bạn sẽ không đăng nhập được nữa. Hãy giữ lại một cách đăng nhập cho bạn.";
const LOCKED_ADMINS =
  "Với chính sách này, không quản trị viên nào còn cách đăng nhập. Hãy giữ lại ít nhất một cách cho họ.";

beforeEach(() => {
  contentWidth(1080);
  signInAsAdmin();
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

const google = () => screen.findByRole("switch", { name: "Đăng nhập bằng Google" });
const password = () =>
  screen.getByRole("switch", { name: "Đăng nhập bằng email và mật khẩu" });
const domains = () =>
  screen.getByRole("textbox", { name: "Tên miền Google được phép" });
const save = () => screen.getByRole("button", { name: "Lưu thay đổi" });
const chips = () =>
  [...document.querySelectorAll('[data-slot="chip"]')].map((chip) => chip.textContent);

async function openSignIn() {
  const view = renderSettings("/admin/settings/sign-in");
  await google();
  return view;
}

describe("the Sign-in section", () => {
  it("shows the four switches with the deck's hints and the Google domains as chips", async () => {
    servesSettings();
    servesImpact();
    await openSignIn();

    for (const [label, hint] of [
      ["Đăng nhập bằng Google", "Liên kết với tài khoản có sẵn qua email đã xác minh"],
      ["Đăng nhập bằng email và mật khẩu", "Chỉ tắt khi mọi người đều dùng Google"],
      [
        "Bắt buộc đổi mật khẩu ở lần đăng nhập đầu",
        "Áp dụng cho tài khoản tạo với mật khẩu tạm",
      ],
      ["Học viên có thể vào lớp bằng mã", "Giáo viên chia sẻ mã hoặc liên kết"],
    ] as const) {
      const row = screen.getByRole("switch", { name: label });
      expect(row).toBeChecked();
      expect(row).toHaveAccessibleDescription(hint);
    }
    expect(chips()).toEqual(["quizzivy.com", "gmail.com"]);
    expect(screen.queryByText(SENTENCE)).toBeNull();
  });

  it("keeps a switch that the document turns off off", async () => {
    servesSettings({ ...SAVED, signIn: { ...SAVED.signIn, joinCodes: false } });
    servesImpact();
    await openSignIn();

    expect(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    ).not.toBeChecked();
  });
});

describe("the Google domains", () => {
  it("adds a domain on Enter, a comma or a space, lower-cased and without a leading @", async () => {
    servesSettings({ ...SAVED, signIn: { ...SAVED.signIn, allowedGoogleDomains: [] } });
    servesImpact();
    await openSignIn();
    const user = userEvent.setup();

    await user.type(domains(), "Example.COM{Enter}");
    await user.type(domains(), "@school.edu.vn,");
    await user.type(domains(), "@@third.org ");

    expect(chips()).toEqual(["example.com", "school.edu.vn", "third.org"]);
    expect(screen.getByText(SENTENCE)).toBeVisible();
  });

  it("adds nothing twice, whatever the case or the @", async () => {
    servesSettings();
    servesImpact();
    await openSignIn();
    const user = userEvent.setup();

    await user.type(domains(), "@QUIZZIVY.com{Enter}");
    await user.type(domains(), "gmail.com,");

    expect(chips()).toHaveLength(2);
    expect(screen.queryByText(SENTENCE)).toBeNull();
  });

  it("removes the last domain on Backspace in the empty input, and any domain with its button", async () => {
    servesSettings();
    servesImpact();
    await openSignIn();
    const user = userEvent.setup();

    await user.click(domains());
    await user.keyboard("{Backspace}");
    expect(chips()).toEqual(["quizzivy.com"]);

    await user.click(screen.getByRole("button", { name: "Bỏ tên miền quizzivy.com" }));
    expect(chips()).toEqual([]);
    expect(domains()).toHaveFocus();
  });

  it("refuses a name that is no domain on its field, and sends nothing", async () => {
    const stored = servesSettings();
    servesImpact();
    await openSignIn();
    const user = userEvent.setup();

    await user.type(domains(), "localhost{Enter}");
    await user.click(save());

    expect(await screen.findByText("Tên miền không hợp lệ.")).toBeVisible();
    expect(domains()).toHaveAttribute("aria-invalid", "true");
    expect(stored.puts).toHaveLength(0);
  });

  it("refuses a domain over 253 characters on its field, and takes one of exactly 253", async () => {
    const stored = servesSettings();
    servesImpact();
    await openSignIn();
    const user = userEvent.setup();
    const labels = (last: number) =>
      [..."abc"]
        .map((letter) => letter.repeat(63))
        .concat("d".repeat(last))
        .join(".");

    await user.click(domains());
    await user.paste(`${labels(62)},`);
    await user.click(save());

    expect(await screen.findByText("Tên miền không hợp lệ.")).toBeVisible();
    expect(domains()).toHaveAttribute("aria-invalid", "true");
    expect(stored.puts).toHaveLength(0);

    await user.click(screen.getByRole("button", { name: `Bỏ tên miền ${labels(62)}` }));
    await user.click(domains());
    await user.paste(`${labels(61)},`);
    await user.click(save());

    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]?.signIn.allowedGoogleDomains).toContain(labels(61));
  });

  it("sends the domains as they are kept", async () => {
    const stored = servesSettings();
    servesImpact();
    await openSignIn();
    const user = userEvent.setup();
    await user.type(domains(), "@School.EDU.vn{Enter}");

    await user.click(save());

    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]?.signIn.allowedGoogleDomains).toEqual([
      "quizzivy.com",
      "gmail.com",
      "school.edu.vn",
    ]);
  });
});

describe("how many people a sign-in policy would lock out", () => {
  it("asks the server about the edited policy a moment after the last edit, and warns", async () => {
    servesSettings();
    const asked = servesImpact({ ...NO_IMPACT, affectedUsers: 3, affectedAdmins: 1 });
    await openSignIn();
    const user = userEvent.setup();
    expect(asked).toHaveLength(0);

    await user.click(screen.getByRole("button", { name: "Bỏ tên miền gmail.com" }));

    expect(
      await screen.findByText("3 người sẽ không đăng nhập được nữa."),
    ).toBeVisible();
    expect(screen.getByText("Trong đó có 1 quản trị viên.")).toBeVisible();
    expect(asked).toEqual([
      { ...SAVED.signIn, allowedGoogleDomains: ["quizzivy.com"] },
    ]);
    expect(save()).not.toHaveAttribute("aria-disabled", "true");
  });

  it("asks once for a burst of edits, about the last one", async () => {
    servesSettings();
    const asked = servesImpact({ ...NO_IMPACT, affectedUsers: 2, affectedAdmins: 0 });
    await openSignIn();
    const user = userEvent.setup({ delay: null });

    await user.type(domains(), "a.com,b.com,c.com,");

    expect(
      await screen.findByText("2 người sẽ không đăng nhập được nữa."),
    ).toBeVisible();
    expect(asked).toHaveLength(1);
    expect(asked[0]?.allowedGoogleDomains).toEqual([
      "quizzivy.com",
      "gmail.com",
      "a.com",
      "b.com",
      "c.com",
    ]);
  });

  it("says nothing and asks nothing while Sign-in equals what is saved", async () => {
    servesSettings();
    const asked = servesImpact({ ...NO_IMPACT, affectedUsers: 5, affectedAdmins: 0 });
    await openSignIn();
    const user = userEvent.setup();

    await user.click(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    );
    await user.click(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    );
    await new Promise((resolve) => setTimeout(resolve, 700));

    expect(asked).toHaveLength(0);
    expect(screen.queryByText(/người sẽ không đăng nhập được nữa/)).toBeNull();
  });

  it("draws nothing for a policy that costs no one a way in", async () => {
    servesSettings();
    const asked = servesImpact(NO_IMPACT);
    await openSignIn();
    const user = userEvent.setup();

    await user.click(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    );
    await waitFor(() => expect(asked).toHaveLength(1));

    expect(screen.queryByText(/người sẽ không đăng nhập được nữa/)).toBeNull();
    expect(save()).not.toHaveAttribute("aria-disabled", "true");
  });
});

describe("a sign-in policy that would lock people out", () => {
  it("pauses Save changes and says why when no method is left, without asking the server", async () => {
    const stored = servesSettings();
    const asked = servesImpact();
    await openSignIn();
    const user = userEvent.setup();

    await user.click(await google());
    await user.click(password());

    expect(save()).toHaveAttribute("aria-disabled", "true");
    expect(save()).toHaveAccessibleDescription(NO_METHOD);
    expect(screen.getAllByText(NO_METHOD)).toHaveLength(2);
    await user.click(save());
    await new Promise((resolve) => setTimeout(resolve, 600));
    expect(stored.puts).toHaveLength(0);
    expect(asked).toHaveLength(0);
  });

  it("puts Save changes back as soon as a method is on again", async () => {
    servesSettings();
    servesImpact();
    await openSignIn();
    const user = userEvent.setup();
    await user.click(await google());
    await user.click(password());
    expect(save()).toHaveAttribute("aria-disabled", "true");

    await user.click(password());

    expect(save()).not.toHaveAttribute("aria-disabled", "true");
    expect(screen.queryByText(NO_METHOD)).toBeNull();
  });

  it("pauses Save changes when the server counts the caller among those locked out", async () => {
    const stored = servesSettings();
    servesImpact({
      affectedUsers: 4,
      affectedAdmins: 1,
      actorLocked: true,
      lockout: true,
    });
    await openSignIn();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Bỏ tên miền gmail.com" }));

    expect(
      await screen.findByText("4 người sẽ không đăng nhập được nữa."),
    ).toBeVisible();
    expect(screen.getAllByText(LOCKED_YOU)).toHaveLength(2);
    expect(save()).toHaveAttribute("aria-disabled", "true");
    await user.click(save());
    expect(stored.puts).toHaveLength(0);
  });

  it("says that no Admin would be left when that is what the server counts", async () => {
    servesSettings();
    servesImpact({
      affectedUsers: 2,
      affectedAdmins: 2,
      actorLocked: false,
      lockout: true,
    });
    await openSignIn();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Bỏ tên miền gmail.com" }));

    expect(await screen.findAllByText(LOCKED_ADMINS)).toHaveLength(2);
    expect(save()).toHaveAttribute("aria-disabled", "true");
  });

  it("shows the server's refusal of a save as the same pause, until the policy changes", async () => {
    const stored = servesSettings(SAVED, () =>
      contractJson(
        "/admin/settings",
        "put",
        422,
        failure("SIGN_IN_POLICY_LOCKOUT", "Chính sách này khóa tất cả quản trị viên.", {
          affectedUsers: 2,
          affectedAdmins: 2,
        }),
      ),
    );
    servesImpact();
    await openSignIn();
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    );

    await user.click(save());

    expect(
      await screen.findAllByText("Chính sách này khóa tất cả quản trị viên."),
    ).not.toHaveLength(0);
    expect(save()).toHaveAttribute("aria-disabled", "true");
    expect(stored.puts).toHaveLength(1);
    await user.click(save());
    expect(stored.puts).toHaveLength(1);

    await user.click(
      screen.getByRole("switch", {
        name: "Bắt buộc đổi mật khẩu ở lần đăng nhập đầu",
      }),
    );

    await waitFor(() => expect(save()).not.toHaveAttribute("aria-disabled", "true"));
    expect(screen.queryByText("Chính sách này khóa tất cả quản trị viên.")).toBeNull();
  });
});

describe("while a save is out", () => {
  it("keeps the switches and Save changes focusable but ignores a press on them", async () => {
    let release: () => void = () => undefined;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    const stored = servesSettings(SAVED, async ({ expectedRevision, ...body }) => {
      await held;
      return contractJson("/admin/settings", "put", 200, {
        ...body,
        revision: expectedRevision + 1,
        updatedAt: "2026-10-15T09:00:00Z",
        updatedBy: SAVED.updatedBy,
      });
    });
    servesImpact();
    await openSignIn();
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("switch", { name: "Học viên có thể vào lớp bằng mã" }),
    );

    await user.click(save());
    const joinCodes = screen.getByRole("switch", {
      name: "Học viên có thể vào lớp bằng mã",
    });
    expect(joinCodes).toHaveAttribute("aria-disabled", "true");
    expect(joinCodes).not.toBeDisabled();
    await user.click(joinCodes);
    await user.click(screen.getByRole("button", { name: "Đang lưu…" }));
    release();

    await waitFor(() => expect(screen.queryByText(SENTENCE)).toBeNull());
    expect(joinCodes).not.toBeChecked();
    expect(stored.puts).toHaveLength(1);
  });
});
