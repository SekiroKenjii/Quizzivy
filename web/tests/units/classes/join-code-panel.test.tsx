import { describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { JoinCodePanel } from "@/features/classes/components/JoinCodePanel";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import type { components } from "@/lib/api/schema";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const CLASS_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const FULL_CODE = "K7M3-P9QR";

const klass: components["schemas"]["Class"] = {
  id: CLASS_ID,
  name: "Tiếng Anh giao tiếp — Lớp A",
  studentCount: 12,
  selfJoinEnabled: true,
  createdAt: "2026-01-01T00:00:00Z",
  joinCode: {
    hint: "P9QR",
    expiresAt: "2026-09-27T00:00:00Z",
    maxUses: 40,
    usesCount: 3,
  },
};

function renderPanel(value = klass) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <JoinCodePanel klass={value} />
    </QueryClientProvider>,
  );
}

describe("the join code panel", () => {
  it("shows no full code until a rotation produces one", () => {
    // §13.3: the plaintext exists once, in the response that created it. Only
    // a SHA-256 hash is stored, so there is no endpoint that COULD return it --
    // this asserts the panel does not pretend otherwise.
    renderPanel();

    const body = document.body.textContent ?? "";
    expect(body).not.toContain(FULL_CODE);
    expect(body).not.toContain("K7M3");
    // The hint is all that survives, and it is masked.
    expect(screen.getByText(/••••-P9QR/)).toBeInTheDocument();
    // And there is no QR to scan, because there is no link to encode.
    expect(document.querySelector("svg")).toBeNull();
  });

  it("shows the counters the teacher needs to judge a code", () => {
    renderPanel();
    expect(screen.getByText("3 / 40")).toBeInTheDocument();
    expect(screen.getByText("27/09/2026")).toBeInTheDocument();
  });

  it("says the class is closed when there is no active code", () => {
    const { joinCode: _omitted, ...closed } = klass;
    renderPanel(closed as components["schemas"]["Class"]);
    expect(screen.getByText(/chưa mở tham gia bằng mã/)).toBeInTheDocument();
  });
});

describe("rotating", () => {
  it("does nothing until the confirmation is accepted", async () => {
    // §6.4 requires the dialog because rotation is not undoable and it takes
    // effect for everyone holding the old code, immediately.
    let rotations = 0;
    server.use(
      http.post(`${BASE}/admin/classes/:id/join-code`, () => {
        rotations += 1;
        return contractJson("/admin/classes/{id}/join-code", "post", 201, {
          code: FULL_CODE,
          expiresAt: "2026-09-27T00:00:00Z",
          maxUses: 40,
        });
      }),
    );

    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText(/Mã cũ sẽ ngừng hoạt động ngay/)).toBeInTheDocument();
    expect(rotations, "opening the dialog must not rotate anything").toBe(0);

    // Backing out leaves the code alone.
    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(rotations).toBe(0);
  });

  it("reveals the code exactly once, with a QR, after confirming", async () => {
    server.use(
      http.post(`${BASE}/admin/classes/:id/join-code`, () =>
        contractJson("/admin/classes/{id}/join-code", "post", 201, {
          code: FULL_CODE,
          expiresAt: "2026-09-27T00:00:00Z",
          maxUses: 40,
        }),
      ),
    );

    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    // The panel button and the dialog's confirm share a name; scope to the
    // dialog so the test presses the one that actually rotates.
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Tạo mã mới",
      }),
    );

    expect(await screen.findByText(FULL_CODE)).toBeInTheDocument();
    expect(screen.getByText(/chỉ hiện một lần/)).toBeInTheDocument();
    // The QR encodes the join LINK, not the bare code: a phone camera opens it.
    expect(document.querySelector("svg")).not.toBeNull();
  });

  it("copies the join link rather than the bare code", async () => {
    server.use(
      http.post(`${BASE}/admin/classes/:id/join-code`, () =>
        contractJson("/admin/classes/{id}/join-code", "post", 201, {
          code: FULL_CODE,
          expiresAt: "2026-09-27T00:00:00Z",
          maxUses: 40,
        }),
      ),
    );

    // userEvent.setup() installs a working clipboard stub; reading it back is
    // simpler and more honest than replacing it with a spy it would overwrite.
    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Tạo mã mới",
      }),
    );
    await screen.findByText(FULL_CODE);

    await user.click(screen.getByRole("button", { name: "Sao chép đường dẫn" }));

    // A link a student can follow, with the dash stripped so it matches the URL
    // shape the router expects -- not the bare code, which is useless in a
    // message on its own.
    await waitFor(async () => {
      expect(await navigator.clipboard.readText()).toMatch(/\/join\/K7M3P9QR$/);
    });
  });
});
