import { it, expect, expectTypeOf } from "vitest";
import {
  fieldErrors,
  matchesOption,
  missingValue,
  optionalField,
  visibleFields,
} from "@/components/shared/form/fields/rules";
import type { FormField } from "@/components/shared/form/FormDialog";
type Values = {
  title: string;
  hidden: string;
  flag: boolean;
  choices: readonly string[];
  file: File | null;
};
const values: Values = { title: " ", hidden: "", flag: false, choices: [], file: null };
const fields: FormField<Values>[] = [
  { kind: "text", name: "title", label: "Tên", required: true },
  { kind: "text", name: "hidden", required: true, when: (v) => v.flag },
  { kind: "toggle", name: "flag", text: "Hiện" },
];
it("ties field names to compatible value types", () => {
  expectTypeOf<
    Extract<FormField<Values>, { kind: "toggle" }>["name"]
  >().toEqualTypeOf<"flag">();
  expectTypeOf<
    Extract<FormField<Values>, { kind: "file" }>["name"]
  >().toEqualTypeOf<"file">();
});
it.each(["", " ", [], false, null])("recognizes missing %j", (v) =>
  expect(missingValue(v)).toBe(true),
);
it.each(["x", ["x"], true, new File(["x"], "a")])("recognizes present %j", (v) =>
  expect(missingValue(v)).toBe(false),
);
it("filters both required and caller validation errors by visible field names", () => {
  expect(visibleFields(fields, values).map((f) => f.name)).toEqual(["title", "flag"]);
  expect(fieldErrors(fields, values, "Bắt buộc", { hidden: "Không hợp lệ" })).toEqual({
    title: "Bắt buộc",
  });
  expect(
    fieldErrors(fields, { ...values, title: "Hợp lệ", flag: true }, "Bắt buộc", {
      title: "Sai",
    }),
  ).toEqual({ title: "Sai", hidden: "Bắt buộc" });
});
it("uses requiredText and never requires info or QR", () => {
  expect(
    fieldErrors(
      [
        { kind: "text", name: "title", required: true, requiredText: "Nhập tên" },
        {
          kind: "qr",
          name: "hidden",
          required: true,
          payload: "x",
          value: "x",
          text: "x",
        },
      ],
      values,
      "Bắt buộc",
    ),
  ).toEqual({ title: "Nhập tên" });
});
it("folds Vietnamese labels and meta with a trimmed query", () => {
  expect(
    matchesOption(
      { value: "1", label: "Nguyễn Gia Bảo", meta: "Đang làm" },
      " nguyen ",
    ),
  ).toBe(true);
  expect(
    matchesOption({ value: "1", label: "Khác", meta: "Đang làm" }, "dang lam"),
  ).toBe(true);
  expect(matchesOption({ value: "1", label: "Nguyễn" }, "Trần")).toBe(false);
});
it("limits optional labels to the six drawn control kinds", () => {
  for (const kind of ["text", "email", "date", "time", "area", "file"] as const) {
    const field =
      kind === "file"
        ? { kind, name: "file" as const }
        : { kind, name: "title" as const };
    expect(optionalField<Values>(field)).toBe(true);
  }
  expect(optionalField<Values>({ kind: "number", name: "title" })).toBe(false);
  expect(optionalField<Values>({ ...fields[0]!, required: true })).toBe(false);
  expect(
    optionalField<Values>({ ...fields[0]!, required: false, noOptional: true }),
  ).toBe(false);
});
