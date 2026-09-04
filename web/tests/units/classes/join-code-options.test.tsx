import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { http } from "msw";
import { JoinCodePanel } from "@/features/classes/components/JoinCodePanel";
import { fetchClass } from "@/features/classes/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import type { components } from "@/lib/api/schema";
import "@/lib/i18n";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const BASE = "http://localhost:8080";
const CLASS_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const FULL_CODE = "K7M3-P9QR";
const NOW = new Date("2026-08-29T00:00:00Z");
const DAY_MS = 86_400_000;

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
  openAssignmentCount: 0,
  archivedAt: null,
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

function Connected() {
  const detail = useQuery({
    queryKey: ["admin-class", CLASS_ID],
    queryFn: () => fetchClass(CLASS_ID),
  });
  return detail.data ? <JoinCodePanel klass={detail.data} /> : null;
}

function renderFromServer() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <Connected />
    </QueryClientProvider>,
  );
}

function captureRotation() {
  const sent: Record<string, unknown>[] = [];
  server.use(
    http.post(`${BASE}/admin/classes/:id/join-code`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      sent.push(body);
      const days =
        typeof body["expiresInDays"] === "number" ? body["expiresInDays"] : 30;
      const maxUses = typeof body["maxUses"] === "number" ? body["maxUses"] : 40;
      return contractJson("/admin/classes/{id}/join-code", "post", 201, {
        code: FULL_CODE,
        expiresAt: new Date(NOW.getTime() + days * DAY_MS).toISOString(),
        maxUses,
      });
    }),
  );
  return sent;
}

async function confirmRotation(user: ReturnType<typeof userEvent.setup>) {
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", {
      name: "Tạo mã mới",
    }),
  );
}

describe("choosing how long a new code lives and how far it goes", () => {
  it("posts exactly the expiry and the limit the teacher picked", async () => {
    const sent = captureRotation();
    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    await user.selectOptions(
      await screen.findByRole("combobox", { name: "Hết hạn sau" }),
      "90",
    );
    await user.type(
      screen.getByRole("spinbutton", { name: "Giới hạn lượt dùng" }),
      "25",
    );
    await confirmRotation(user);

    await screen.findByText(FULL_CODE);
    expect(sent).toEqual([{ expiresInDays: 90, maxUses: 25 }]);
  });

  it("sends no maxUses at all when the limit is left empty", async () => {
    const sent = captureRotation();
    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    await confirmRotation(user);

    await screen.findByText(FULL_CODE);
    expect(sent).toHaveLength(1);
    expect(sent[0]).not.toHaveProperty("maxUses");
    expect(sent[0]).toEqual({ expiresInDays: 30 });
  });

  it("refuses a limit the contract would reject, rather than posting it", async () => {
    const sent = captureRotation();
    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    const limit = await screen.findByRole("spinbutton", {
      name: "Giới hạn lượt dùng",
    });
    await user.type(limit, "1001");

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("alert")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Tạo mã mới" })).toBeDisabled();

    await user.clear(limit);
    await user.type(limit, "40");
    expect(within(dialog).queryByRole("alert")).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Tạo mã mới" }));

    await screen.findByText(FULL_CODE);
    expect(sent).toEqual([{ expiresInDays: 30, maxUses: 40 }]);
  });

  it("forgets the previous choice the next time the dialog opens", async () => {
    const sent = captureRotation();
    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    await user.type(
      screen.getByRole("spinbutton", { name: "Giới hạn lượt dùng" }),
      "25",
    );
    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());

    await user.click(screen.getByRole("button", { name: "Tạo mã mới" }));
    await confirmRotation(user);

    await screen.findByText(FULL_CODE);
    expect(sent).toEqual([{ expiresInDays: 30 }]);
  });
});

describe("the card after a code is issued", () => {
  it("states the expiry and the usage the server came back with", async () => {
    let current = klass.joinCode ?? null;
    server.use(
      http.get(`${BASE}/admin/classes/:id`, () =>
        contractJson("/admin/classes/{id}", "get", 200, {
          ...klass,
          joinCode: current,
        }),
      ),
      http.post(`${BASE}/admin/classes/:id/join-code`, async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        const days = body["expiresInDays"] as number;
        const maxUses = (body["maxUses"] as number | undefined) ?? 40;
        const expiresAt = new Date(NOW.getTime() + days * DAY_MS).toISOString();
        current = { hint: "P9QR", expiresAt, maxUses, usesCount: 0 };
        return contractJson("/admin/classes/{id}/join-code", "post", 201, {
          code: FULL_CODE,
          expiresAt,
          maxUses,
        });
      }),
    );

    const user = userEvent.setup();
    renderFromServer();

    await user.click(await screen.findByRole("button", { name: "Tạo mã mới" }));
    await user.selectOptions(
      await screen.findByRole("combobox", { name: "Hết hạn sau" }),
      "90",
    );
    await user.type(
      screen.getByRole("spinbutton", { name: "Giới hạn lượt dùng" }),
      "25",
    );
    await confirmRotation(user);

    await screen.findByText(FULL_CODE);
    await user.click(screen.getByRole("button", { name: "Xong" }));

    expect(await screen.findByText("0 / 25")).toBeInTheDocument();
    expect(screen.getByText(/27\/11\/2026/)).toBeInTheDocument();
  });
});

describe("a code that has run out of uses", () => {
  const spent: components["schemas"]["Class"] = {
    ...klass,
    joinCode: {
      hint: "P9QR",
      expiresAt: "2026-09-27T00:00:00Z",
      maxUses: 40,
      usesCount: 40,
    },
  };

  it("says so instead of looking live", () => {
    renderPanel(spent);
    expect(screen.getByText("đã dùng hết")).toBeInTheDocument();
    expect(screen.getByText(/đã hết lượt dùng/)).toBeInTheDocument();
    expect(screen.getByText("40 / 40")).toBeInTheDocument();
  });

  it("offers to issue a replacement, not to rotate something usable", () => {
    renderPanel(spent);
    expect(
      screen.getByRole("button", { name: "Mở tham gia bằng mã" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Tạo mã mới" })).toBeNull();
  });
});

describe("a code issued before the cap existed", () => {
  const uncapped: components["schemas"]["Class"] = {
    ...klass,
    joinCode: {
      hint: "P9QR",
      expiresAt: "2026-09-27T00:00:00Z",
      maxUses: null,
      usesCount: 3,
    },
  };

  it("reads as unlimited and never as spent, however often it was used", () => {
    renderPanel(uncapped);
    expect(screen.getByText("3 / không giới hạn")).toBeInTheDocument();
    expect(screen.queryByText("đã dùng hết")).toBeNull();
    expect(screen.getByRole("button", { name: "Tạo mã mới" })).toBeInTheDocument();
  });
});
