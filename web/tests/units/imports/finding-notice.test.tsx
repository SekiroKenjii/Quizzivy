import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FindingNotice } from "@/features/imports/components/review/FindingNotice";
import type { ImportFinding } from "@/features/imports/api";
import { finding } from "./fixtures";
import "@/lib/i18n";

function show(value: ImportFinding) {
  render(
    <FindingNotice
      finding={value}
      current={false}
      onAcknowledge={() => undefined}
      onLocate={() => undefined}
    />,
  );
}

describe("a PDF note in the review", () => {
  it("names repeated lines that were left out, and asks for a decision", () => {
    show(
      finding({
        id: "f1",
        code: "UNSUPPORTED_DOCUMENT_OBJECT",
        field: "PDF_REPEATED_LINE_REQUIRES_REVIEW",
        severity: "review_required",
        count: 2,
        evidence: [{ sourceId: "s", blockId: "p1-l1", start: 0, end: 12 }],
      }),
    );
    expect(
      screen.getByText("2 dòng lặp lại ở đầu hoặc cuối trang đã được bỏ ra"),
    ).toBeInTheDocument();
    expect(screen.getByText(/hãy thêm lại khi rà soát/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Xác nhận đã kiểm tra" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/đối tượng trong tài liệu không đọc được/)).toBeNull();
  });

  it("says a PDF keeps no formatting rather than that an object went unread", () => {
    show(
      finding({
        id: "f2",
        code: "UNSUPPORTED_DOCUMENT_OBJECT",
        field: "PDF_MARKS_UNAVAILABLE",
        severity: "informational",
      }),
    );
    expect(screen.getByText("PDF không giữ định dạng chữ")).toBeInTheDocument();
    expect(screen.queryByText(/đối tượng trong tài liệu không đọc được/)).toBeNull();
  });

  it("says pasted text keeps no formatting, and why that matters", () => {
    show(
      finding({
        id: "f4",
        code: "UNSUPPORTED_DOCUMENT_OBJECT",
        field: "TEXT_MARKS_UNAVAILABLE",
        severity: "informational",
      }),
    );
    expect(screen.getByText("Văn bản dán không giữ định dạng chữ")).toBeInTheDocument();
    expect(
      screen.getByText(/^Văn bản thuần không giữ được gạch chân/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/đối tượng trong tài liệu không đọc được/)).toBeNull();
  });

  it("says how many pages the AI service read, as information that needs no decision", () => {
    show(
      finding({
        id: "f5",
        code: "UNSUPPORTED_DOCUMENT_OBJECT",
        field: "AI_PAGE_READ",
        severity: "informational",
        count: 12,
      }),
    );
    expect(
      screen.getByText("12 trang đã được một dịch vụ AI bên ngoài Quizzivy đọc"),
    ).toBeInTheDocument();
    expect(screen.getByText(/đối chiếu các câu với trang gốc/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Xác nhận đã kiểm tra" })).toBeNull();
    expect(screen.queryByText(/đối tượng trong tài liệu không đọc được/)).toBeNull();
  });

  it("asks for a decision about a page the AI service could not read", () => {
    show(
      finding({
        id: "f6",
        code: "UNSUPPORTED_DOCUMENT_OBJECT",
        field: "AI_PAGE_UNREADABLE",
        severity: "review_required",
        count: 1,
        evidence: [{ sourceId: "s", blockId: "p3-l1", start: 0, end: 5 }],
      }),
    );
    expect(screen.getByText("1 trang AI không đọc được")).toBeInTheDocument();
    expect(screen.getByText(/Hãy thêm các câu đó bằng tay/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Xác nhận đã kiểm tra" }),
    ).toBeInTheDocument();
  });

  it("keeps the generic title for a Word object", () => {
    show(
      finding({
        id: "f3",
        code: "UNSUPPORTED_DOCUMENT_OBJECT",
        field: "TEXTBOX_ORDER_REQUIRES_REVIEW",
        severity: "informational",
      }),
    );
    expect(
      screen.getByText("1 đối tượng trong tài liệu không đọc được"),
    ).toBeInTheDocument();
  });
});
