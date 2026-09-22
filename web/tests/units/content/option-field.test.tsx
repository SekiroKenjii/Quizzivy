import { render, screen } from "@testing-library/react";
import { afterEach } from "vitest";
import { OptionField } from "@/features/question-bank/components/OptionField";
import { OptionsEditor } from "@/features/question-bank/components/OptionsEditor";
import { plainOptionContent } from "@/components/shared/content/optionContent";
import "@/lib/i18n";

afterEach(() => vi.unstubAllEnvs());

test("answer selection and option text have distinct accessible names", () => {
  render(
    <OptionsEditor
      options={[
        { id: null, text: "think", isCorrect: true },
        { id: null, text: "other", isCorrect: false },
      ]}
      multiple={false}
      fixed={false}
      onChange={vi.fn()}
    />,
  );
  expect(screen.getByLabelText("Lựa chọn 1", { exact: true })).toHaveAttribute(
    "type",
    "radio",
  );
  expect(
    screen.getByRole("textbox", { name: "Nội dung lựa chọn 1", exact: true }),
  ).toHaveValue("think");
});

test("the pilot flag gates new formatting while existing content remains editable", () => {
  vi.stubEnv("VITE_RICH_OPTION_EDITOR", "false");
  const change = vi.fn();
  const view = render(<OptionField text="plain" index={0} onChange={change} />);
  expect(
    screen.queryByRole("button", { name: "Định dạng phương án 1" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("textbox", { name: "Nội dung lựa chọn 1" })).toHaveValue(
    "plain",
  );
  view.rerender(
    <OptionField
      text="rich"
      content={plainOptionContent("rich")}
      index={0}
      onChange={change}
    />,
  );
  expect(
    screen.getByRole("button", { name: "Sửa định dạng phương án 1" }),
  ).toBeEnabled();
  expect(change).not.toHaveBeenCalled();
});

test("pilot authoring exposes an explicitly named formatting control", () => {
  vi.stubEnv("VITE_RICH_OPTION_EDITOR", "true");
  render(<OptionField text="plain" index={0} onChange={vi.fn()} />);
  expect(screen.getByRole("button", { name: "Định dạng phương án 1" })).toBeEnabled();
});
