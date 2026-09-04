import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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

/**
 * The clock is pinned because the panel compares the fixture's `expiresAt`
 * against it: `expired` decides whether the primary button says "Tạo mã mới" or
 * "Mở tham gia bằng mã", so on 27/09/2026 this file would have started failing
 * eight of its tests, and the expired-code block would have gone green for the
 * wrong reason.
 */
const NOW = new Date("2026-08-29T00:00:00Z");

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: NOW });
});

afterEach(() => {
  vi.useRealTimers();
});

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
    renderPanel();

    const body = document.body.textContent ?? "";
    expect(body).not.toContain(FULL_CODE);
    expect(body).not.toContain("K7M3");
    // The hint is all that survives, and it is masked.
    expect(screen.getByText(/••••-P9QR/)).toBeInTheDocument();
    // And there is no QR to scan, because there is no link to encode.
    expect(screen.queryByLabelText("Mã QR dẫn tới trang tham gia lớp")).toBeNull();
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
    await waitFor(async () => {
      expect(await navigator.clipboard.readText()).toMatch(/\/join\/K7M3P9QR$/);
    });
  });
});

describe("an expired code", () => {
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
    renderPanel(expiredClass);
    expect(screen.getByText(/16:00, 01\/01\/2020/)).toBeInTheDocument();
  });
});

describe("revoking", () => {
  it("asks first, like rotating does", async () => {
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

    await user.click(screen.getByRole("button", { name: "Xong" }));
    await waitFor(() => expect(screen.queryByText(FULL_CODE)).not.toBeInTheDocument());

    await user.click(screen.getByRole("button", { name: "Ngừng cho tham gia" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Ngừng cho tham gia",
      }),
    );

    await waitFor(() => expect(screen.queryByText(FULL_CODE)).not.toBeInTheDocument());
    expect(document.body.textContent).not.toContain("K7M3");
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
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    // And no code is claimed to exist.
    expect(screen.queryByText(FULL_CODE)).toBeNull();
  });

  it("clears a stale error once something else succeeds", async () => {
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
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Tạo mã mới",
      }),
    );
    await screen.findByText(FULL_CODE);
    return screen.getByRole("button", { name: "Sao chép đường dẫn" });
  }

  it("explains a rejected clipboard write instead of going quiet", async () => {
    const user = userEvent.setup();
    const copy = await rotateThenGetCopyButton(user);

    const restore = withClipboard({
      writeText: () => Promise.reject(new Error("denied")),
    });
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

/** Same rule as the temporary password: the code is shown once, so Esc is not a way out. */
describe("the fresh code dialog", () => {
  it("ignores Escape and closes only through Xong", async () => {
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
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Tạo mã mới",
      }),
    );
    expect(await screen.findByText(FULL_CODE)).toBeInTheDocument();

    await user.keyboard("{Escape}");
    expect(screen.getByText(FULL_CODE)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Xong" }));
    expect(screen.queryByText(FULL_CODE)).toBeNull();
  });
});
