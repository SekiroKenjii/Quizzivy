import { describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { JoinCodesOffCallout } from "@/features/classes/components/JoinCodesOffCallout";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
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
      <JoinCodesOffCallout />
    </QueryClientProvider>,
  );
}

describe("JoinCodesOffCallout", () => {
  it("says the centre has turned joining by code off, in the page's language", async () => {
    server.use(organization(false));
    await i18n.changeLanguage("vi");
    renderCallout();
    expect(
      await screen.findByText("Trung tâm đang tắt tham gia bằng mã."),
    ).toBeInTheDocument();
    await i18n.changeLanguage("en");
    expect(
      await screen.findByText("Your centre has turned off joining with a code."),
    ).toBeInTheDocument();
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
