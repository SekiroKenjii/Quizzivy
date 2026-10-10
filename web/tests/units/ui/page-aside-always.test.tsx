import { describe, expect, it } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { PageAside } from "@/components/shared/PageAside";
import { viewport } from "@tests/support/viewport";
import "@/lib/i18n";

function Content() {
  const [draft, setDraft] = useState("");
  return (
    <label>
      Private draft
      <input value={draft} onChange={(event) => setDraft(event.target.value)} />
    </label>
  );
}

describe("PageAside explicit constant-dialog mode", () => {
  it.each([undefined, false])(
    "preserves the default wide inline column with always=%s",
    (always) => {
      viewport(1440);
      render(
        <PageAside
          label="Settings"
          sheet={{
            open: true,
            onOpenChange: () => {},
            ...(always === undefined ? {} : { always }),
          }}
        >
          <Content />
        </PageAside>,
      );
      expect(
        screen.getByRole("complementary", { name: "Settings" }),
      ).toBeInTheDocument();
      expect(screen.queryByRole("dialog")).toBeNull();
    },
  );
  it("keeps its open dialog subtree and draft mounted from wide to phone and back", () => {
    const display = viewport(1440);
    render(
      <PageAside
        label="Settings"
        sheet={{ open: true, onOpenChange: () => {}, always: true }}
      >
        <Content />
      </PageAside>,
    );
    const dialog = screen.getByRole("dialog", { name: "Settings" });
    const input = screen.getByLabelText("Private draft");
    fireEvent.change(input, { target: { value: "unfinished" } });
    for (const width of [360, 768, 1024, 1440]) {
      act(() => display.resize(width));
      expect(screen.getByRole("dialog")).toBe(dialog);
      expect(screen.getByLabelText("Private draft")).toBe(input);
      expect(input).toHaveValue("unfinished");
      expect(screen.queryByRole("complementary")).toBeNull();
    }
  });
});
