import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { toast } from "sonner";

import "@/lib/i18n";
import { Toaster } from "@/components/ui/sonner";
import { notify } from "@/lib/toast";

afterEach(() => {
  act(() => {
    toast.dismiss();
  });
});

async function show(fire: () => void) {
  render(<Toaster />);
  act(() => {
    fire();
  });
  return screen.findByText(
    (_, element) => element?.hasAttribute("data-sonner-toast") ?? false,
  );
}

describe("toast tones", () => {
  it.each([
    ["success", "text-success"],
    ["info", "text-info-ink"],
    ["warning", "text-warning-ink"],
    ["error", "text-danger-ink"],
  ] as const)("gives %s its own icon colour", async (tone, colour) => {
    const card = await show(() => notify[tone]("Đã lưu thay đổi"));
    expect(card.querySelector("[data-icon] svg")).toHaveClass(colour);
  });

  it("announces an error at once, and nothing else that way", async () => {
    await show(() => notify.error("Không lưu được. Thử lại sau."));
    expect(screen.getByRole("alert")).toHaveTextContent("Không lưu được. Thử lại sau.");
  });

  it("keeps success and information polite and brief", async () => {
    await show(() => notify.success("Đã sao chép mã lớp"));
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByRole("button", { name: "Đóng thông báo" })).toBeNull();
  });

  it("lets warnings and errors be closed by name", async () => {
    await show(() => notify.warning("Còn 5 phút"));
    expect(screen.getByRole("button", { name: "Đóng thông báo" })).toBeInTheDocument();
  });
});
