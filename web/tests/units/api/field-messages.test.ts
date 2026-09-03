import { describe, expect, it } from "vitest";
import { ApiError, fieldMessages } from "@/lib/api/errors";

function validationError(details?: Record<string, unknown>) {
  return new ApiError({
    status: 400,
    code: "VALIDATION_FAILED",
    message: "Dữ liệu bài giao không hợp lệ.",
    ...(details ? { details } : {}),
  });
}

/**
 * The envelope's `message` names no field. A form that renders only it leaves
 * the teacher to guess which of a dozen inputs the server objected to, while
 * the server has already written the exact sentence.
 */
describe("the per-field reasons behind a validation failure", () => {
  it("returns the sentences the server wrote", () => {
    expect(
      fieldMessages(
        validationError({
          "window.closesAt": "Thời điểm đóng phải sau thời điểm mở.",
          targets: "Chọn ít nhất một lớp hoặc một học viên.",
        }),
      ),
    ).toEqual([
      "Thời điểm đóng phải sau thời điểm mở.",
      "Chọn ít nhất một lớp hoặc một học viên.",
    ]);
  });

  it("is empty when the response carried no details", () => {
    expect(fieldMessages(validationError())).toEqual([]);
  });

  it("drops anything that is not a sentence", () => {
    expect(
      fieldMessages(validationError({ a: "real", b: { nested: 1 }, c: 7 })),
    ).toEqual(["real"]);
  });

  it("ignores errors that are not ApiErrors at all", () => {
    expect(fieldMessages(new Error("network down"))).toEqual([]);
    expect(fieldMessages(undefined)).toEqual([]);
  });
});
