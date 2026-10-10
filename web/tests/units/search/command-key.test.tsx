import { render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { CommandKey } from "@/features/search/CommandKey";
import { commandKeyLabel } from "@/features/search/useCommandPalette";
import "@/lib/i18n";

afterEach(() => {
  vi.restoreAllMocks();
  Reflect.deleteProperty(navigator, "userAgentData");
});

function platform(name: string, userAgentData?: string) {
  vi.spyOn(navigator, "platform", "get").mockReturnValue(name);
  if (userAgentData !== undefined)
    Object.defineProperty(navigator, "userAgentData", {
      configurable: true,
      value: { platform: userAgentData },
    });
}

test.each([
  ["Win32", undefined, "Ctrl"],
  ["Linux x86_64", "Linux", "Ctrl"],
  ["MacIntel", undefined, "Cmd"],
  ["MacIntel", "macOS", "Cmd"],
  ["iPad", undefined, "Cmd"],
])("names the modifier on %s (userAgentData %s) as %s", (name, data, label) => {
  platform(name, data);
  expect(commandKeyLabel()).toBe(label);
});

test("draws ⌘ as a named icon on an Apple device and Ctrl as text elsewhere", () => {
  platform("MacIntel", "macOS");
  const mac = render(<CommandKey />);
  expect(screen.getByRole("img", { name: "Command" })).toBeInTheDocument();
  expect(mac.container.textContent).toBe("");
  mac.unmount();
  platform("Win32", "Windows");
  const other = render(<CommandKey />);
  expect(other.container.textContent).toBe("Ctrl");
});
