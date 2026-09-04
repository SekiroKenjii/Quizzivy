import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { STATUS_BADGES, StatusBadge } from "@/components/shared/StatusBadge";
import "@/lib/i18n";

describe("the status vocabulary", () => {
  it("gives every state exactly one badge, as F-07 draws them", () => {
    expect(STATUS_BADGES).toEqual({
      test: { draft: "secondary", published: "success", archived: "outline" },
      assignment: {
        draft: "secondary",
        scheduled: "outline",
        open: "success",
        closed: "secondary",
      },
      attempt: {
        not_started: "outline",
        in_progress: "primary",
        submitted: "secondary",
        timed_out: "warning",
        graded: "success",
        voided: "danger",
      },
      attention: { flagged: "warning", pendingManual: "outline", audio: "outline" },
    });
  });

  it("names the state in the deck's words", () => {
    render(
      <>
        <StatusBadge kind="assignment" status="scheduled" />
        <StatusBadge kind="attempt" status="timed_out" />
      </>,
    );
    expect(screen.getByText("Đã lên lịch")).toBeInTheDocument();
    expect(screen.getByText("Hết giờ")).toBeInTheDocument();
  });
});
