import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toApiError } from "@/lib/api/errors";
import { PublishDialog } from "@/features/tests/components/PublishDialog";
import "@/lib/i18n";

it("preserves the contract's top-level violations and offers section and question jumps", async () => {
  const error = await toApiError(
    new Response(
      JSON.stringify({
        error: { code: "PUBLISH_VALIDATION_FAILED", message: "Cannot publish" },
        violations: [
          { rule: "section_not_empty", message: "Empty section", sectionId: "s1" },
          {
            rule: "choice_has_correct_option",
            message: "No correct option",
            questionId: "q1",
            sectionId: "s2",
          },
        ],
      }),
      { status: 409 },
    ),
  );
  const question = vi.fn();
  const section = vi.fn();
  const publish = vi.fn();
  render(
    <PublishDialog
      open
      pending={false}
      error={null}
      problems={[]}
      violations={error.violations}
      onOpenChange={vi.fn()}
      onGoTo={question}
      onGoToSection={section}
      onPublish={publish}
      warnings={[{ questionId: "q2", message: "Explanation missing" }]}
    />,
  );
  expect(screen.getByText("Empty section")).toBeVisible();
  expect(screen.getByText("No correct option")).toBeVisible();
  expect(screen.getByText("Explanation missing")).toBeVisible();
  const buttons = screen.getAllByRole("button", { name: "Sửa" });
  await userEvent.click(buttons[0]!);
  await userEvent.click(buttons[1]!);
  expect(section).toHaveBeenCalledWith("s1");
  expect(question).toHaveBeenCalledWith("q1");
  await userEvent.click(screen.getByRole("button", { name: "Phát hành" }));
  expect(
    await screen.findByText("Hãy sửa các mục trên trước khi phát hành."),
  ).toBeVisible();
  expect(publish).not.toHaveBeenCalled();
});
