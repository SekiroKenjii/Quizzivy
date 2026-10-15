import { afterEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { http, HttpResponse } from "msw";
import { JoinCodesOffCallout } from "@/features/classes/components/JoinCodesOffCallout";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import i18n from "@/lib/i18n";

const BASE = "http://localhost:8080";

function organization(joinCodes: boolean, onRead: () => void = () => {}) {
  return http.get(`${BASE}/public/organization`, () => {
    onRead();
    return contractJson("/public/organization", "get", 200, {
      name: "Quizzivy",
      shortName: null,
      website: null,
      contactEmail: null,
      frontDeskPhone: null,
      frontDeskHours: null,
      address: null,
      logo: { lightUrl: null, darkUrl: null },
      signIn: { google: true, password: true },
      joinCodes,
    });
  });
}

function renderCallout() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <JoinCodesOffCallout />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

afterEach(() => {
  useAuthStore.setState({ user: null });
});

describe("JoinCodesOffCallout", () => {
  it("says the centre has turned joining by code off, in the page's language", async () => {
    server.use(organization(false));
    await i18n.changeLanguage("vi");
    renderCallout();
    expect(
      await screen.findByText(/Trung tâm đang tắt tham gia bằng mã\./),
    ).toBeInTheDocument();
    await i18n.changeLanguage("en");
    expect(
      await screen.findByText(/Your centre has turned off joining with a code\./),
    ).toBeInTheDocument();
  });

  it("links a holder of system.settings.write to the sign-in settings", async () => {
    server.use(organization(false));
    useAuthStore.setState({ user: adminUser });
    await i18n.changeLanguage("en");
    renderCallout();
    const link = await screen.findByRole("link", { name: "Open sign-in settings" });
    expect(link).toHaveAttribute("href", "/admin/settings/sign-in");
    expect(screen.queryByText(/Ask your admin/)).toBeNull();
  });

  it("tells anyone else to ask their admin, with no link", async () => {
    server.use(organization(false));
    useAuthStore.setState({ user: teacherUser });
    await i18n.changeLanguage("vi");
    renderCallout();
    expect(await screen.findByText(/Hãy hỏi quản trị viên\./)).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
    await i18n.changeLanguage("en");
    expect(await screen.findByText(/Ask your admin\./)).toBeInTheDocument();
  });

  it("shows nothing while the centre allows joining by code", async () => {
    let asked = false;
    server.use(
      organization(true, () => {
        asked = true;
      }),
    );
    renderCallout();
    await waitFor(() => expect(asked).toBe(true));
    expect(screen.queryByText(/joining with a code|tham gia bằng mã/)).toBeNull();
  });

  it("shows nothing when the centre cannot be read", async () => {
    server.use(
      http.get(`${BASE}/public/organization`, () =>
        HttpResponse.json(
          {
            error: {
              code: "INTERNAL",
              message: "Lỗi.",
              requestId: "018f0000-0000-7000-8000-0000000000f1",
            },
          },
          { status: 500 },
        ),
      ),
    );
    renderCallout();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(screen.queryByRole("note")).toBeNull();
    expect(screen.queryByText(/joining with a code|tham gia bằng mã/)).toBeNull();
  });
});
