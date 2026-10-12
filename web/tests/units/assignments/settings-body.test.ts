import { describe, expect, it } from "vitest";
import { draftOf } from "@/features/assignments/draft";
import { settingsBody } from "@/features/assignments/settingsBody";
import { detailAssignment } from "@tests/support/assignmentDetail";

describe("settingsBody", () => {
  it("sends an unchanged window as stored, to the second", () => {
    const a = detailAssignment();
    const body = settingsBody("window", a, draftOf(a, []), "open");
    expect(body.window).toEqual({
      opensAt: a.window.opensAt,
      closesAt: a.window.closesAt,
    });
  });

  it("sends a moved close as the minute chosen in the window's zone", () => {
    const a = detailAssignment();
    const draft = { ...draftOf(a, []), closesAt: "2099-09-10T21:00" };
    const body = settingsBody("window", a, draft, "open");
    expect(body.window).toEqual({
      opensAt: a.window.opensAt,
      closesAt: "2099-09-10T14:00:00.000Z",
    });
  });

  it("keeps a draft a draft, and leaves the timing alone outside its group", () => {
    const a = detailAssignment({ status: "draft", publishedAt: null });
    const draft = { ...draftOf(a, []), durationMinutes: 90 };
    expect(settingsBody("results", a, draft, "draft")).toMatchObject({
      draft: true,
      durationMinutes: 45,
    });
    expect(settingsBody("timing", a, draft, "draft")).toMatchObject({
      draft: true,
      durationMinutes: 90,
    });
  });

  it("clears a note that is only whitespace", () => {
    const a = detailAssignment({ studentNote: "Mang tai nghe" });
    const draft = { ...draftOf(a, []), studentNote: "   " };
    expect(settingsBody("note", a, draft, "open").studentNote).toBeNull();
  });
});
