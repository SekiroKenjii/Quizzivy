import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  DeckDialog,
  DeckDialogActions,
  DeckDialogCancel,
} from "@/components/shared/DeckDialog";
import "@/lib/i18n";

function open(placement?: "center" | "top") {
  render(
    <DeckDialog
      open
      onOpenChange={() => {}}
      title="Bắt đầu ngay?"
      description="Mô tả"
      {...(placement === undefined ? {} : { placement })}
    >
      <form onSubmit={(event) => event.preventDefault()}>
        <DeckDialogActions>
          <DeckDialogCancel>Để sau</DeckDialogCancel>
        </DeckDialogActions>
      </form>
    </DeckDialog>,
  );
  return screen.getByRole("dialog", { name: "Bắt đầu ngay?" });
}

describe("the student dialog's frame", () => {
  it("is centred unless it is asked to sit near the top", () => {
    const frame = open();
    expect(frame).toHaveClass("top-[50%]");
    expect(frame).not.toHaveClass("top-[12%]");
  });

  it("sits 12% from the top below 768 when it holds a field", () => {
    const frame = open("top");
    expect(frame).toHaveClass("top-[12%]", "min-[768px]:top-[50%]");
    expect(frame).not.toHaveClass("top-[50%]");
  });

  it("draws a flat secondary button that does not submit a form", () => {
    open();
    const cancel = screen.getByRole("button", { name: "Để sau" });
    expect(cancel).toHaveClass("shadow-none");
    expect(cancel).toHaveAttribute("type", "button");
  });
});
