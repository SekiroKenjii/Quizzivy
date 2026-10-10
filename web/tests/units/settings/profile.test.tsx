import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import { ProfileSection } from "@/features/settings/sections/Profile";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { BASE, REQUEST_ID, signIn } from "./support";
import "@/lib/i18n";

const SAVED = {
  ...teacherUser,
  fullName: "Hoàng Thương",
  displayName: "Ms Thương",
  phone: "0912 345 678",
  locale: "vi" as const,
  timeZone: "Asia/Ho_Chi_Minh",
};

beforeEach(() => {
  contentWidth(1080);
  signIn(SAVED);
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

function renderProfile() {
  render(
    <>
      <ProfileSection />
      <Toaster />
    </>,
  );
}

function patches(answer: (body: Record<string, unknown>) => Response) {
  const bodies: Record<string, unknown>[] = [];
  server.use(
    http.patch(`${BASE}/auth/me`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      bodies.push(body);
      return answer(body);
    }),
  );
  return bodies;
}

const field = (name: string) => screen.getByRole("textbox", { name });

describe("the Profile card", () => {
  it("draws the deck's fields, hints and a read-only email", () => {
    renderProfile();

    expect(screen.getByRole("heading", { name: "Hồ sơ" })).toBeVisible();
    expect(screen.getByText("Học viên thấy tên và ảnh của bạn.")).toBeVisible();
    expect(field("Họ và tên")).toHaveValue("Hoàng Thương");
    expect(field("Tên học viên thấy")).toHaveValue("Ms Thương");
    expect(field("Tên học viên thấy")).toHaveAccessibleDescription(
      "Hiện trên đề thi và tin nhắn",
    );
    expect(field("Email")).toHaveValue(teacherUser.email);
    expect(field("Email")).toHaveAttribute("readonly");
    expect(field("Email")).toHaveAccessibleDescription("Dùng để đăng nhập");
    expect(field("Số điện thoại")).toHaveValue("0912 345 678");
    expect(screen.getByRole("combobox", { name: "Múi giờ" })).toHaveTextContent(
      /^\(GMT\+7\) TP\. Hồ Chí Minh$/,
    );
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
  });

  it("raises the DirtyBar on an edit, and Discard restores what was saved", async () => {
    renderProfile();
    const user = userEvent.setup();

    await user.type(field("Họ và tên"), " Hoàng");
    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));

    expect(field("Họ và tên")).toHaveValue("Hoàng Thương");
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
  });

  it("saves only what changed, clears a blank field with null, and says Settings saved", async () => {
    const bodies = patches((body) =>
      contractJson("/auth/me", "patch", 200, {
        ...SAVED,
        ...body,
        displayName: undefined,
      }),
    );
    renderProfile();
    const user = userEvent.setup();

    await user.clear(field("Tên học viên thấy"));
    await user.click(screen.getByRole("combobox", { name: "Múi giờ" }));
    await user.click(screen.getByRole("option", { name: /Tokyo$/ }));
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    expect(await screen.findByText("Đã lưu cài đặt.")).toBeInTheDocument();
    expect(bodies).toEqual([{ displayName: null, timeZone: "Asia/Tokyo" }]);
    expect(field("Tên học viên thấy")).toHaveValue("");
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    expect(useAuthStore.getState().user?.timeZone).toBe("Asia/Tokyo");
  });

  it("refuses an empty name and a phone outside the rule without asking the server", async () => {
    const bodies = patches(() => contractJson("/auth/me", "patch", 200, SAVED));
    renderProfile();
    const user = userEvent.setup();

    await user.clear(field("Họ và tên"));
    await user.clear(field("Số điện thoại"));
    await user.type(field("Số điện thoại"), "09-12");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    expect(await screen.findByText("Họ và tên không được để trống.")).toBeVisible();
    expect(field("Số điện thoại")).toHaveAccessibleDescription(
      "Số điện thoại gồm 6–20 ký tự: chữ số, dấu + và khoảng trắng.",
    );
    expect(bodies).toEqual([]);
  });

  it("keeps the edit and shows the server's refusal in the bar", async () => {
    patches(() =>
      HttpResponse.json(
        {
          error: {
            code: "VALIDATION_FAILED",
            message: "Múi giờ không hợp lệ.",
            requestId: REQUEST_ID,
          },
        },
        { status: 400 },
      ),
    );
    renderProfile();
    const user = userEvent.setup();

    await user.type(field("Họ và tên"), " Hoàng");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Múi giờ không hợp lệ.");
    expect(field("Họ và tên")).toHaveValue("Hoàng Thương Hoàng");
  });

  it("shows an account zone outside the curated list under its own name", () => {
    signIn({ ...SAVED, timeZone: "America/Chicago" });
    renderProfile();

    expect(screen.getByRole("combobox", { name: "Múi giờ" })).toHaveTextContent(
      /America\/Chicago$/,
    );
  });
});

describe("the profile photo", () => {
  const PNG = new File([new Uint8Array(32)], "me.png", { type: "image/png" });

  async function choose(file: File) {
    const user = userEvent.setup({ applyAccept: false });
    await user.click(screen.getByRole("button", { name: "Đổi ảnh" }));
    const dialog = await screen.findByRole("dialog", { name: "Đổi ảnh" });
    const input = dialog.querySelector<HTMLInputElement>('input[type="file"]')!;
    await user.upload(input, file);
    await user.click(within(dialog).getByRole("button", { name: "Lưu ảnh" }));
    return dialog;
  }

  it("shows the initials while there is no photo, and no Remove", () => {
    renderProfile();

    expect(screen.getByText("TH")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Gỡ ảnh" })).toBeNull();
  });

  it("uploads a new photo with PUT and shows it at once", async () => {
    const methods: string[] = [];
    server.use(
      http.put(`${BASE}/me/avatar`, ({ request }) => {
        methods.push(request.method);
        return contractJson("/me/avatar", "put", 200, {
          ...SAVED,
          avatarUrl: "https://media.example.com/avatar.png",
        });
      }),
    );
    renderProfile();

    await choose(PNG);

    expect(await screen.findByText("Đã cập nhật ảnh.")).toBeInTheDocument();
    expect(methods).toEqual(["PUT"]);
    expect(screen.getByRole("img", { name: "Ảnh hồ sơ của bạn" })).toHaveAttribute(
      "src",
      "https://media.example.com/avatar.png",
    );
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
  });

  it("refuses a file of the wrong type before sending it", async () => {
    const sent: string[] = [];
    server.use(
      http.put(`${BASE}/me/avatar`, () => {
        sent.push("put");
        return contractJson("/me/avatar", "put", 200, SAVED);
      }),
    );
    renderProfile();

    const dialog = await choose(new File(["gif"], "me.gif", { type: "image/gif" }));

    expect(await within(dialog).findByText("Chỉ nhận ảnh PNG hoặc JPG.")).toBeVisible();
    expect(sent).toEqual([]);
  });

  it("refuses a file over 2 MiB before sending it", async () => {
    renderProfile();

    const dialog = await choose(
      new File([new Uint8Array(2 * 1024 * 1024 + 1)], "big.png", { type: "image/png" }),
    );

    expect(await within(dialog).findByText("Ảnh tối đa 2 MB.")).toBeVisible();
  });

  it.each([
    [413, "MEDIA_TOO_LARGE", "Ảnh lớn hơn 2 MiB."],
    [415, "MEDIA_TYPE_UNSUPPORTED", "Chỉ nhận PNG hoặc JPEG."],
    [415, "IMAGE_DIMENSIONS", "Mỗi cạnh của ảnh phải từ 200 đến 2048 điểm ảnh."],
  ])("shows the server's message for a %i %s", async (status, code, message) => {
    server.use(
      http.put(`${BASE}/me/avatar`, () =>
        contractJson("/me/avatar", "put", status, {
          error: { code, message, requestId: REQUEST_ID },
        }),
      ),
    );
    renderProfile();

    const dialog = await choose(PNG);

    expect(await within(dialog).findByText(message)).toBeVisible();
  });

  it("removes the photo after a confirmation", async () => {
    signIn({ ...SAVED, avatarUrl: "https://media.example.com/avatar.png" });
    let removed = 0;
    server.use(
      http.delete(`${BASE}/me/avatar`, () => {
        removed += 1;
        return contractJson("/me/avatar", "delete", 200, SAVED);
      }),
    );
    renderProfile();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Gỡ ảnh" }));
    const dialog = await screen.findByRole("dialog", { name: "Gỡ ảnh của bạn?" });
    expect(
      within(dialog).getByText("Học viên sẽ thấy chữ cái đầu tên bạn thay cho ảnh."),
    ).toBeVisible();
    expect(removed).toBe(0);
    await user.click(within(dialog).getByRole("button", { name: "Gỡ ảnh" }));

    expect(await screen.findByText("Đã gỡ ảnh.")).toBeInTheDocument();
    expect(removed).toBe(1);
    await waitFor(() => expect(screen.getByText("TH")).toBeVisible());
  });
});
