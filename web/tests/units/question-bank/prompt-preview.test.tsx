import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { Markdown } from "@/components/shared/Markdown";
import { blankSlots } from "@/features/question-bank/blankSlots";

function preview(prompt: string) {
  const { container } = render(<Markdown plugins={[blankSlots]}>{prompt}</Markdown>);
  return container;
}

describe("the fill_blank placeholder renderer", () => {
  it("renders each {{n}} as a numbered slot and leaves the prose alone", () => {
    const container = preview("If it {{1}} tomorrow, we {{2}} the trip.");

    const slots = container.querySelectorAll("[data-blank]");
    expect(slots).toHaveLength(2);
    expect(slots[0]).toHaveAttribute("data-blank", "1");
    expect(slots[1]).toHaveAttribute("data-blank", "2");
    expect(container.textContent).toContain("tomorrow, we");
  });

  it("renders Markdown as markup, not as literal text", () => {
    const container = preview("She **lives** in {{1}}.");

    expect(container.querySelector("strong")).toHaveTextContent("lives");
    expect(container.textContent).not.toContain("**");
  });

  it("sanitises a prompt: a teacher's Markdown is read by every student", () => {
    const container = preview(
      '<img src=x onerror="alert(1)"> <script>alert(2)</script> plain {{1}}',
    );

    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("[onerror]")).toBeNull();
    expect(container.innerHTML).not.toContain("onerror");
    expect(container.querySelectorAll("[data-blank]")).toHaveLength(1);
  });

  it("leaves text with no placeholder untouched", () => {
    const container = preview("Nothing to fill in here.");

    expect(container.querySelectorAll("[data-blank]")).toHaveLength(0);
    expect(screen.getByText("Nothing to fill in here.")).toBeInTheDocument();
  });
});
