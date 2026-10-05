import { expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeckScale } from "@/components/ui/deck-scale";
import { cases } from "@tests/support/deck-harness/cases/r4-02b";
import "@/lib/i18n";

it("keeps the Extend switch checked state independent of its legitimate help tooltip trigger", async () => {
  const Extend = cases["form-extend"]!;
  const user = userEvent.setup();
  render(
    <DeckScale>
      <Extend />
    </DeckScale>,
  );
  const toggle = screen.getByRole("switch", { name: "Notify students" });
  expect(toggle).toBeChecked();
  expect(toggle).toHaveAttribute("data-state", "checked");
  await user.click(screen.getByText("Notify students", { selector: "span" }));
  expect(toggle).not.toBeChecked();
  expect(toggle).toHaveAttribute("data-state", "unchecked");
  act(() => toggle.focus());
  await user.keyboard(" ");
  expect(toggle).toBeChecked();
  expect(toggle).toHaveAttribute("data-state", "checked");
  await user.keyboard("{Enter}");
  expect(toggle).not.toBeChecked();
  expect(toggle).toHaveAttribute("data-state", "unchecked");
  const help = screen.getByRole("button", { name: "About notification delivery" });
  act(() => help.focus());
  const tip = await screen.findByRole("tooltip");
  expect(tip).toHaveTextContent("They get an in-app and email message");
  expect(help).toHaveAccessibleDescription("They get an in-app and email message");
  expect(toggle).toHaveAttribute("data-state", "unchecked");
  await user.tab();
  expect(toggle).toHaveFocus();
});
