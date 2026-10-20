import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { useAuthStore } from "@/stores/auth";
import { stubDrawing, pngFile, svgFile, type Drawing } from "./drawing";
import {
  BASE,
  DARK_HASH,
  HASH,
  failure,
  servesImpact,
  servesLogoUpload,
  servesPublicOrganization,
  servesSettings,
  signInAsAdmin,
} from "./support";
import { renderSettings } from "./render";
import "@/lib/i18n";

const SENTENCE = "Bạn có thay đổi chưa lưu.";
const TOAST = "Logo đã tải lên. Lưu cài đặt để áp dụng.";

let drawing: Drawing | undefined;

beforeEach(() => {
  contentWidth(1080);
  signInAsAdmin();
  servesImpact();
  drawing = stubDrawing();
});

afterEach(() => {
  vi.restoreAllMocks();
  drawing?.restore();
  drawing = undefined;
  useAuthStore.getState().clearSession();
});

const opener = () => screen.findByRole("button", { name: "Thay logo" });
const dialog = () => screen.findByRole("dialog", { name: "Thay logo" });
const files = () => [
  ...document.querySelectorAll<HTMLInputElement>('input[type="file"]'),
];
const lightInput = () => files()[0]!;
const darkInput = () => files()[1]!;
const submit = () => screen.getByRole("button", { name: "Lưu logo" });
const tile = () => screen.queryByRole("img", { name: "Logo của trung tâm" });

async function openDialog(user: ReturnType<typeof userEvent.setup>) {
  renderSettings();
  await user.click(await opener());
  return within(await dialog());
}

describe("Replace logo", () => {
  it("asks for a logo for a light ground and offers one for a dark ground", async () => {
    servesSettings();
    const user = userEvent.setup();

    const modal = await openDialog(user);

    expect(modal.getByText("Logo")).toBeVisible();
    expect(modal.getByText("Bản cho chế độ tối")).toBeVisible();
    expect(modal.getByText("SVG hoặc PNG vuông, tối thiểu 256 × 256")).toBeVisible();
    expect(modal.getByText("Logo sáng trên nền trong suốt")).toBeVisible();
    expect(files()).toHaveLength(2);
    expect(lightInput()).toHaveAttribute("accept", expect.stringContaining(".svg"));
    expect(lightInput()).toHaveAttribute(
      "accept",
      expect.stringContaining("image/png"),
    );
  });

  it("will not send until a logo is chosen", async () => {
    servesSettings();
    const sent = servesLogoUpload();
    const user = userEvent.setup();
    await openDialog(user);

    await user.click(submit());

    expect(await screen.findByText("Bắt buộc")).toBeVisible();
    expect(sent).toHaveLength(0);
    expect(await dialog()).toBeVisible();
  });

  it("uploads a PNG as the light logo, shows it before the save, and says it is not applied yet", async () => {
    servesSettings();
    const sent = servesLogoUpload();
    const user = userEvent.setup();
    await openDialog(user);

    await user.upload(lightInput(), pngFile("mark.png"));
    await user.click(submit());

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent).toEqual([
      { variant: "light", name: "mark.png", type: "image/png", size: 16 },
    ]);
    expect((await screen.findAllByText(TOAST)).length).toBeGreaterThan(0);
    expect(tile()).toHaveAttribute("src", "blob:logo-1");
    expect(screen.getByText(SENTENCE)).toBeVisible();
    expect(screen.getByText("SVG hoặc PNG, hình vuông")).toBeVisible();
    expect(await opener()).toHaveFocus();
  });

  it("uploads the dark version after the light one, and says the centre has both", async () => {
    servesSettings();
    const sent = servesLogoUpload();
    const user = userEvent.setup();
    await openDialog(user);

    await user.upload(lightInput(), pngFile("sang.png"));
    await user.upload(darkInput(), pngFile("toi.png"));
    await user.click(submit());

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent.map(({ variant, name }) => [variant, name])).toEqual([
      ["light", "sang.png"],
      ["dark", "toi.png"],
    ]);
    expect(
      screen.getByText("SVG hoặc PNG, hình vuông · có bản cho chế độ tối"),
    ).toBeVisible();
  });

  it("draws an SVG to a PNG in the browser and uploads that, named after the SVG", async () => {
    servesSettings();
    const sent = servesLogoUpload();
    const user = userEvent.setup();
    await openDialog(user);

    await user.upload(lightInput(), svgFile(undefined, "mark.svg"));
    await user.click(submit());

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent).toEqual([
      { variant: "light", name: "mark.png", type: "image/png", size: 9 },
    ]);
    expect(drawing?.drawn).toEqual([{ width: 1024, height: 1024 }]);
  });

  it("sends the hashes of the uploaded logos in the save, and none of the pictures", async () => {
    const stored = servesSettings();
    servesLogoUpload();
    servesPublicOrganization();
    const user = userEvent.setup();
    await openDialog(user);
    await user.upload(lightInput(), pngFile("sang.png"));
    await user.upload(darkInput(), pngFile("toi.png"));
    await user.click(submit());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0]?.organization.logo).toEqual({
      light: { sha256: HASH, type: "image/png" },
      dark: { sha256: DARK_HASH, type: "image/png" },
    });
    await waitFor(() => expect(screen.queryByText(SENTENCE)).toBeNull());
    expect(tile()).toHaveAttribute(
      "src",
      `${BASE}/public/organization/logo?variant=light&v=${HASH}`,
    );
  });

  it("refuses a file that is neither a PNG nor an SVG on its field, before anything is sent", async () => {
    servesSettings();
    const sent = servesLogoUpload();
    const user = userEvent.setup({ applyAccept: false });
    await openDialog(user);

    await user.upload(
      lightInput(),
      new File(["x"], "logo.jpg", { type: "image/jpeg" }),
    );
    await user.click(submit());

    expect(await screen.findByText("Hãy chọn tệp SVG hoặc PNG.")).toBeVisible();
    expect(sent).toHaveLength(0);
  });

  it("refuses an SVG that does not draw, and a PNG over 1 MiB", async () => {
    servesSettings();
    const sent = servesLogoUpload();
    const user = userEvent.setup();
    await openDialog(user);

    await user.upload(lightInput(), svgFile("<svg", "hong.svg"));
    await user.click(submit());
    expect(
      await screen.findByText("Không vẽ được tệp này. Hãy chọn tệp SVG hoặc PNG khác."),
    ).toBeVisible();

    await user.upload(lightInput(), pngFile("nang.png", 1024 * 1024 + 1));
    await user.click(submit());
    expect(await screen.findByText("PNG tối đa 1 MiB.")).toBeVisible();
    expect(sent).toHaveLength(0);
  });

  it("refuses an SVG that is not square", async () => {
    servesSettings();
    const sent = servesLogoUpload();
    const user = userEvent.setup();
    await openDialog(user);

    await user.upload(
      lightInput(),
      svgFile(
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 100"><rect/></svg>',
      ),
    );
    await user.click(submit());

    expect(
      await screen.findByText("Logo phải là hình vuông (lệch tối đa 2%)."),
    ).toBeVisible();
    expect(sent).toHaveLength(0);
  });

  it("shows the server's refusal of a file on its field until another file is chosen", async () => {
    servesSettings();
    const sent = servesLogoUpload((variant) =>
      variant === "light" && sent.length === 1
        ? contractJson(
            "/admin/settings/logo",
            "post",
            415,
            failure("IMAGE_DIMENSIONS", "Logo phải có cạnh từ 256 đến 2048 điểm ảnh."),
          )
        : undefined,
    );
    const user = userEvent.setup();
    await openDialog(user);
    await user.upload(lightInput(), pngFile("be.png"));

    await user.click(submit());

    expect(
      await screen.findByText("Logo phải có cạnh từ 256 đến 2048 điểm ảnh."),
    ).toBeVisible();
    expect(await dialog()).toBeVisible();
    expect(tile()).toBeNull();
    expect(screen.queryByText(SENTENCE)).toBeNull();

    await user.click(submit());
    expect(sent).toHaveLength(1);

    await user.upload(lightInput(), pngFile("lon.png"));
    await waitFor(() =>
      expect(
        screen.queryByText("Logo phải có cạnh từ 256 đến 2048 điểm ảnh."),
      ).toBeNull(),
    );
    await user.click(submit());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent.map(({ name }) => name)).toEqual(["be.png", "lon.png"]);
  });

  it("names the dark version's field when it is the dark one that the server refuses", async () => {
    servesSettings();
    const sent = servesLogoUpload((variant) =>
      variant === "dark"
        ? contractJson(
            "/admin/settings/logo",
            "post",
            415,
            failure("MEDIA_UNREADABLE", "Không đọc được ảnh này."),
          )
        : undefined,
    );
    const user = userEvent.setup();
    await openDialog(user);
    await user.upload(lightInput(), pngFile("sang.png"));
    await user.upload(darkInput(), pngFile("toi.png"));

    await user.click(submit());

    expect(await screen.findByText("Không đọc được ảnh này.")).toBeVisible();
    expect(sent.map(({ variant }) => variant)).toEqual(["light", "dark"]);
    expect(await dialog()).toBeVisible();
    expect(tile()).toBeNull();
  });

  it("says what went wrong when the server could not store a logo at all", async () => {
    servesSettings();
    servesLogoUpload(() =>
      contractJson(
        "/admin/settings/logo",
        "post",
        501,
        failure("INTERNAL", "Chưa cấu hình nơi lưu trữ."),
      ),
    );
    const user = userEvent.setup();
    const modal = await openDialog(user);
    await user.upload(lightInput(), pngFile("mark.png"));

    await user.click(submit());

    expect(await modal.findByText("Chưa cấu hình nơi lưu trữ.")).toBeVisible();
    expect(tile()).toBeNull();
  });

  it("closes on Cancel without sending anything, and gives focus back to Replace logo", async () => {
    servesSettings();
    const sent = servesLogoUpload();
    const user = userEvent.setup();
    const modal = await openDialog(user);
    await user.upload(lightInput(), pngFile("mark.png"));

    await user.click(modal.getByRole("button", { name: "Huỷ" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(sent).toHaveLength(0);
    expect(screen.queryByText(SENTENCE)).toBeNull();
    expect(await opener()).toHaveFocus();
  });
});
