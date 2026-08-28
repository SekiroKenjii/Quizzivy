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
    expect(screen.getByText(/07:00, 27\/09\/2026/)).toBeInTheDocument();
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

describe("an expired code", () => {
  // The server keeps an expired code in the active slot on purpose: the partial
  // unique index is on `revoked_at IS NULL`, because only rotation revokes
  // (§6.1). So "there is a code" and "students can use it" are different
  // questions, and the panel used to answer only the first -- showing a healthy
  // code with a uses counter while /join turned every student away.
  const expiredClass: components["schemas"]["Class"] = {
    ...klass,
    joinCode: {
      hint: "P9QR",
      expiresAt: "2020-01-01T09:00:00Z",
      maxUses: 40,
      usesCount: 3,
    },
  };

  it("is labelled expired rather than shown as active", () => {
    renderPanel(expiredClass);
    expect(screen.getByText("đã hết hạn")).toBeInTheDocument();
    expect(screen.getByText(/Mã này đã hết hạn/)).toBeInTheDocument();
  });

  it("offers to ISSUE a code, not to rotate one", () => {
    // Rotating implies replacing something in use. There is nothing in use.
    renderPanel(expiredClass);
    expect(
      screen.getByRole("button", { name: "Mở tham gia bằng mã" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Tạo mã mới" })).toBeNull();
  });

  it("shows the time, not just the date", () => {
    // dd/MM/yyyy renders a code that died at 09:00 identically to one good
    // until 23:59. This is a bearer secret's expiry.
    renderPanel(expiredClass);
    expect(screen.getByText(/16:00, 01\/01\/2020/)).toBeInTheDocument();
  });
});

describe("revoking", () => {
  it("asks first, like rotating does", async () => {
    // Revoking is the harsher of the two: immediate, not undoable, and it
    // issues no replacement. One of the three destructive actions on this
    // screen having a dialog and the others not was an inconsistency.
    let revocations = 0;
    server.use(
      http.delete(`${BASE}/admin/classes/:id/join-code`, () => {
        revocations += 1;
        return new Response(null, { status: 204 });
      }),
    );

    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("button", { name: "Ngừng cho tham gia" }));

    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(revocations, "opening the dialog must not revoke anything").toBe(0);

    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(revocations).toBe(0);
  });

  it("stops displaying a code it has just killed", async () => {
    // The revealed code is dead the moment the server accepts the revoke.
    // Leaving it on screen invites the teacher to hand out something that no
    // longer works.
    server.use(
      http.post(`${BASE}/admin/classes/:id/join-code`, () =>
        contractJson("/admin/classes/{id}/join-code", "post", 201, {
          code: FULL_CODE,
          expiresAt: "2026-09-27T00:00:00Z",
          maxUses: 40,
        }),
      ),
      http.delete(
        `${BASE}/admin/classes/:id/join-code`,
        () => new Response(null, { status: 204 }),
      ),
    );

    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Tạo mã mới",
      }),
    );
    expect(await screen.findByText(FULL_CODE)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Ngừng cho tham gia" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Ngừng cho tham gia",
      }),
    );

    await waitFor(() => expect(screen.queryByText(FULL_CODE)).not.toBeInTheDocument());
  });
});

describe("failures", () => {
  it("tells the teacher when a rotation fails", async () => {
    server.use(
      http.post(
        `${BASE}/admin/classes/:id/join-code`,
        () => new Response(null, { status: 500 }),
      ),
    );

    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Tạo mã mới",
      }),
    );

    // The dialog closes first: Radix marks everything outside an open dialog
    // aria-hidden, so an error rendered in the panel behind it is invisible to
    // sighted users and absent from the accessibility tree.
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    // And no code is claimed to exist.
    expect(screen.queryByText(FULL_CODE)).toBeNull();
  });

  it("clears a stale error once something else succeeds", async () => {
    // rotate.onSuccess cleared the error and revoke.onSuccess did not, so a
    // failed rotate followed by a successful revoke left a red message on
    // screen describing an operation two steps back.
    server.use(
      http.post(
        `${BASE}/admin/classes/:id/join-code`,
        () => new Response(null, { status: 500 }),
      ),
      http.delete(
        `${BASE}/admin/classes/:id/join-code`,
        () => new Response(null, { status: 204 }),
      ),
    );

    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Tạo mã mới",
      }),
    );
    expect(await screen.findByRole("alert")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Ngừng cho tham gia" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Ngừng cho tham gia",
      }),
    );

    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
  });
});

describe("copying the join link", () => {
  // Both failures are ordinary: a denied permission rejects the promise, and on
  // a non-secure origin navigator.clipboard is undefined so reading .writeText
  // throws synchronously. Unguarded, each gives a button that does nothing and
  // says nothing -- and the fallback (select the link and copy it by hand) is
  // only obvious once someone says so.

  // Replaces the clipboard AFTER userEvent.setup(), which installs a working
  // stub of its own that would otherwise win.
  function withClipboard(value: unknown) {
    const original = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", {
      value,
      configurable: true,
      writable: true,
    });
    return () => {
      if (original) Object.defineProperty(navigator, "clipboard", original);
    };
  }

  // The copy button exists only once a rotation has produced a plaintext code.
  async function rotateThenGetCopyButton(user: ReturnType<typeof userEvent.setup>) {
    server.use(
      http.post(`${BASE}/admin/classes/:id/join-code`, () =>
        contractJson("/admin/classes/{id}/join-code", "post", 201, {
          code: FULL_CODE,
          expiresAt: "2026-09-27T00:00:00Z",
          maxUses: 40,
        }),
      ),
    );
    renderPanel();
    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", { name: "Tạo mã mới" }),
    );
    await screen.findByText(FULL_CODE);
    return screen.getByRole("button", { name: "Sao chép đường dẫn" });
  }

  it("explains a rejected clipboard write instead of going quiet", async () => {
    const user = userEvent.setup();
    const copy = await rotateThenGetCopyButton(user);

    const restore = withClipboard({ writeText: () => Promise.reject(new Error("denied")) });
    try {
      await user.click(copy);
      expect(await screen.findByText(/Không sao chép được/i)).toBeInTheDocument();
      // And it must not claim success at the same time.
      expect(screen.queryByRole("button", { name: "Đã sao chép" })).toBeNull();
    } finally {
      restore();
    }
  });

  it("survives a non-secure origin, where navigator.clipboard is undefined", async () => {
    const user = userEvent.setup();
    const copy = await rotateThenGetCopyButton(user);

    const restore = withClipboard(undefined);
    try {
      // The unguarded version threw synchronously reading .writeText.
      await user.click(copy);
      expect(await screen.findByText(/Không sao chép được/i)).toBeInTheDocument();
    } finally {
      restore();
    }
  });
});
