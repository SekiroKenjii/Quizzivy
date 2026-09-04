import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TokenField, type Token } from "@/features/assignments/components/TokenField";
import "@/lib/i18n";

const OPTIONS: Token[] = [
  { id: "c1", label: "Lớp 6A", hint: "24" },
  { id: "c2", label: "Lớp 6B", hint: "22" },
  { id: "c3", label: "Lớp 7A", hint: "30" },
];

function Picker({ onAdd }: { onAdd: (token: Token) => void }) {
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Token[]>([]);
  return (
    <TokenField
      label="Lớp"
      placeholder="thêm lớp"
      selected={selected}
      options={OPTIONS}
      loading={false}
      query={query}
      onQueryChange={setQuery}
      onAdd={(token) => {
        setSelected((current) => [...current, token]);
        onAdd(token);
      }}
      onRemove={(id) =>
        setSelected((current) => current.filter((token) => token.id !== id))
      }
    />
  );
}

describe("the token field, from the keyboard alone", () => {
  it("picks the option the arrows landed on, not the first one", async () => {
    const user = userEvent.setup();
    const added = vi.fn();
    render(<Picker onAdd={added} />);

    await user.tab();
    const input = screen.getByRole("combobox");
    expect(input).toHaveFocus();
    expect(input).not.toHaveAttribute("aria-activedescendant");

    await user.keyboard("{ArrowDown}{ArrowDown}");
    const second = screen.getByRole("option", { name: /Lớp 6B/ });
    expect(input).toHaveAttribute("aria-activedescendant", second.id);
    expect(second).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("option", { name: /Lớp 6A/ })).toHaveAttribute(
      "aria-selected",
      "false",
    );

    await user.keyboard("{Enter}");
    expect(added).toHaveBeenCalledTimes(1);
    expect(added.mock.calls[0]?.[0]).toMatchObject({ id: "c2" });
    expect(screen.getByRole("button", { name: "Bỏ Lớp 6B" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: /Lớp 6B/ })).toBeNull();
    expect(screen.getByRole("combobox")).not.toHaveAttribute("aria-activedescendant");
    expect(screen.getByRole("combobox")).toHaveFocus();

    await user.keyboard("{Backspace}");
    expect(screen.queryByRole("button", { name: "Bỏ Lớp 6B" })).toBeNull();
  });

  it("wraps upwards to the last option", async () => {
    const user = userEvent.setup();
    const added = vi.fn();
    render(<Picker onAdd={added} />);

    await user.tab();
    await user.keyboard("{ArrowUp}");
    const last = screen.getByRole("option", { name: /Lớp 7A/ });
    expect(screen.getByRole("combobox")).toHaveAttribute(
      "aria-activedescendant",
      last.id,
    );

    await user.keyboard("{Enter}");
    expect(added.mock.calls[0]?.[0]).toMatchObject({ id: "c3" });
  });

  it("closes the list on Escape and keeps the caret where it was", async () => {
    const user = userEvent.setup();
    render(<Picker onAdd={() => {}} />);

    await user.tab();
    expect(screen.getByRole("listbox", { name: "Lớp" })).toBeInTheDocument();

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    const input = screen.getByRole("combobox");
    expect(input).toHaveFocus();
    expect(input).toHaveAttribute("aria-expanded", "false");

    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("listbox")).toBeInTheDocument();
  });

  it("gives Home and End to the list while empty, and to the caret once typed", async () => {
    const user = userEvent.setup();
    render(<Picker onAdd={() => {}} />);

    await user.tab();
    const input = screen.getByRole("combobox");

    await user.keyboard("{End}");
    expect(input).toHaveAttribute(
      "aria-activedescendant",
      screen.getByRole("option", { name: /Lớp 7A/ }).id,
    );

    await user.keyboard("{Home}");
    const first = screen.getByRole("option", { name: /Lớp 6A/ });
    expect(input).toHaveAttribute("aria-activedescendant", first.id);

    await user.keyboard("6{End}");
    expect(input).toHaveValue("6");
    expect(input).toHaveAttribute("aria-activedescendant", first.id);
  });
});
