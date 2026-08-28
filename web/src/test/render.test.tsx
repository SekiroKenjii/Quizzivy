import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api/client";
import "@/lib/i18n";

/**
 * End-to-end smoke over the harness: Testing Library renders a component, the
 * component fetches through the real API client, MSW answers with a
 * contract-validated fixture, and i18next supplies Vietnamese copy.
 *
 * If any layer of the harness is mis-wired, this fails — which is the point of
 * having it before any real screen exists.
 */
function ClassList() {
  const { t } = useTranslation();
  const { data, isPending } = useQuery({
    queryKey: ["classes"],
    queryFn: () => api("get", "/app/classes"),
  });

  if (isPending) return <p>{t("common.loading")}</p>;
  return (
    <ul aria-label={t("student.myClasses")}>
      {data?.items.map((c) => <li key={c.id}>{c.name}</li>)}
    </ul>
  );
}

function renderWithProviders(ui: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

describe("harness smoke", () => {
  it("renders data fetched through the client and mocked by MSW", async () => {
    renderWithProviders(<ClassList />);

    // Vietnamese copy from i18next, not a raw key.
    expect(screen.getByText("Đang tải…")).toBeInTheDocument();

    expect(await screen.findByText("Tiếng Anh giao tiếp - Lớp A")).toBeInTheDocument();
    // The aria-label comes from t(), which is what §14 requires.
    expect(screen.getByRole("list", { name: "Lớp của tôi" })).toBeInTheDocument();
  });
});
