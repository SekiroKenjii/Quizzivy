import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { PromptField } from "@/features/question-bank/components/PromptField";
import "@/lib/i18n";

it("clears the starter visually without saving an empty prompt and formats the visible text", async () => {
  const changed = vi.fn();
  function Editor() {
    const [value, setValue] = useState("Câu hỏi mới — nhập nội dung ở đây");
    return (
      <PromptField
        id="starter"
        clearOnFocus
        value={value}
        onChange={(next) => {
          changed(next);
          setValue(next);
        }}
      />
    );
  }
  render(<Editor />);
  const user = userEvent.setup();
  const field = screen.getByRole("textbox");
  await user.click(field);
  expect(field).toHaveValue("");
  expect(changed).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Đậm" }));
  expect(field).toHaveValue("****");
  await user.clear(field);
  await user.type(field, "My own content");
  await user.tab();
  await user.click(field);
  expect(field).toHaveValue("My own content");
});
