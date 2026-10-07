import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/teacher/TestBuilderPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";

const test = {
  id: TEST_ID,
  title: "Unit 5",
  description: null,
  status: "draft" as const,
  currentVersion: 0,
  totalPoints: 1,
  questionCount: 1,
  audioCount: 0,
  sections: [
    {
      id: "018f0000-0000-7000-8000-0000000000c1",
      ordinal: 0,
      title: "Ngữ pháp",
      instructions: null,
      questionIds: [QUESTION_ID],
    },
  ],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-02T00:00:00Z",
};

const question = {
  id: QUESTION_ID,
  type: "single_choice" as const,
  prompt: "They ___ to the museum.",
  media: null,
  audio: null,
  transcript: null,
  options: [
    {
      id: "018f0000-0000-7000-8000-0000000000d1",
      text: "went",
      ordinal: 0,
      isCorrect: true,
    },
    {
      id: "018f0000-0000-7000-8000-0000000000d2",
      text: "have gone",
      ordinal: 1,
      isCorrect: false,
    },
  ],
  blanks: [],
  points: 1,
  explanation: null,
  sampleAnswer: null,
  tags: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

import { viewport } from "@tests/support/viewport";
import { registerContentElement } from "@/layouts/shell/contentWidth";

let patches: { title?: string }[] = [];
beforeEach(() => {
  viewport(1024);
  localStorage.removeItem("quizzivy.builder.outline");
  patches = [];
  server.use(
    http.get(`${BASE}/teacher/tests/:id`, () =>
      contractJson("/teacher/tests/{id}", "get", 200, test),
    ),
    http.get(`${BASE}/teacher/questions/:id`, () =>
      contractJson("/teacher/questions/{id}", "get", 200, question),
    ),
    http.patch(`${BASE}/teacher/questions/:id`, async ({ request }) => {
      const body = (await request.json()) as { prompt: string };
      return contractJson("/teacher/questions/{id}", "patch", 200, {
        ...question,
        prompt: body.prompt,
      });
    }),
    http.patch(`${BASE}/teacher/tests/:id`, async ({ request }) => {
      const body = (await request.json()) as { title?: string };
      patches.push(body);
      return contractJson("/teacher/tests/{id}", "patch", 200, {
        ...test,
        title: body.title ?? test.title,
      });
    }),
  );
});
afterEach(() => {
  registerContentElement(null);
  vi.restoreAllMocks();
});
function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/teacher/tests/${TEST_ID}/edit`] },
  );
  return {
    ...render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    ),
    user: userEvent.setup(),
  };
}

describe("the deck builder frame", () => {
  it("opens the actual title input with the keyboard and commits trimmed text on Enter", async () => {
    const { user } = renderBuilder();
    const trigger = await screen.findByRole("button", { name: "Tên đề thi" });
    trigger.focus();
    await user.keyboard("{Enter}");
    const input = screen.getByRole("textbox", { name: "Tên đề thi" });
    expect(input).toHaveFocus();
    await user.clear(input);
    await user.type(input, "  Revised title  {Enter}");
    expect(screen.queryByRole("textbox", { name: "Tên đề thi" })).toBeNull();
    expect(screen.getByRole("button", { name: "Tên đề thi" })).toHaveTextContent(
      "Revised title",
    );
    await waitFor(() => expect(patches.at(-1)?.title).toBe("Revised title"), {
      timeout: 3000,
    });
  });
  it("blurs on Escape and uses the real Untitled fallback for empty text", async () => {
    const { user } = renderBuilder();
    await user.click(await screen.findByRole("button", { name: "Tên đề thi" }));
    const input = screen.getByRole("textbox", { name: "Tên đề thi" });
    await user.clear(input);
    await user.keyboard("{Escape}");
    expect(screen.getByRole("button", { name: "Tên đề thi" })).toHaveTextContent(
      "Đề thi chưa đặt tên",
    );
    await waitFor(() => expect(patches.at(-1)?.title).toBe("Đề thi chưa đặt tên"), {
      timeout: 3000,
    });
  });
  it("needs both viewport and actual content thresholds and retains the mounted editor", async () => {
    const view = viewport(768);
    const area = document.createElement("main");
    Object.defineProperty(area, "offsetWidth", { value: 594 });
    document.body.append(area);
    act(() => registerContentElement(area));
    const { user } = renderBuilder();
    const input = await screen.findByLabelText("Nội dung câu hỏi");
    await user.type(input, " retained draft");
    expect(input).toHaveValue(`${question.prompt} retained draft`);
    expect(
      screen.getByRole("separator", { name: "Độ rộng cấu trúc" }),
    ).toBeInTheDocument();
    act(() => view.resize(767));
    expect(screen.queryByRole("separator", { name: "Độ rộng cấu trúc" })).toBeNull();
    expect(screen.getByLabelText("Nội dung câu hỏi")).toBe(input);
    act(() => view.resize(1024));
    const small = document.createElement("main");
    Object.defineProperty(small, "offsetWidth", { value: 593 });
    act(() => registerContentElement(small));
    expect(screen.queryByRole("separator", { name: "Độ rộng cấu trúc" })).toBeNull();
    expect(screen.getByLabelText("Nội dung câu hỏi")).toBe(input);
    act(() => registerContentElement(area));
    expect(
      screen.getByRole("separator", { name: "Độ rộng cấu trúc" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Nội dung câu hỏi")).toBe(input);
    expect(input).toHaveValue(`${question.prompt} retained draft`);
    area.remove();
  });
  it("clamps remembered outline width to content minus 374 and resets its own key", async () => {
    localStorage.setItem("quizzivy.builder.outline", "900");
    renderBuilder();
    await screen.findByRole("button", { name: "Tên đề thi" });
    const grip = screen.getByRole("separator", { name: "Độ rộng cấu trúc" });
    vi.spyOn(grip.parentElement!, "getBoundingClientRect").mockReturnValue({
      width: 720,
      left: 0,
      right: 720,
      top: 0,
      bottom: 500,
      height: 500,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    });
    fireEvent.focus(grip);
    expect(grip).toHaveAttribute("aria-valuenow", "346");
    fireEvent.keyDown(grip, { key: "ArrowLeft" });
    expect(grip).toHaveAttribute("aria-valuenow", "330");
    expect(localStorage.getItem("quizzivy.builder.outline")).toBe("330");
    fireEvent.doubleClick(grip);
    expect(grip).toHaveAttribute("aria-valuenow", "300");
    expect(localStorage.getItem("quizzivy.builder.outline")).toBeNull();
  });
});

it("keeps the composing title focused until an ordinary Enter commits it", async () => {
  const { user } = renderBuilder();
  await user.click(await screen.findByRole("button", { name: "Tên đề thi" }));
  const input = screen.getByRole("textbox", { name: "Tên đề thi" });
  fireEvent.compositionStart(input);
  fireEvent.change(input, { target: { value: "  Tiếng Việt  " } });
  fireEvent.keyDown(input, { key: "Enter", code: "Enter", isComposing: true });
  expect(screen.getByRole("textbox", { name: "Tên đề thi" })).toBe(input);
  expect(input).toHaveFocus();
  expect(input).toHaveValue("  Tiếng Việt  ");
  await waitFor(() => expect(patches.at(-1)?.title).toBe("  Tiếng Việt  "), {
    timeout: 3000,
  });
  fireEvent.compositionEnd(input);
  await user.keyboard("{Enter}");
  expect(screen.queryByRole("textbox", { name: "Tên đề thi" })).toBeNull();
  const trigger = screen.getByRole("button", { name: "Tên đề thi" });
  expect(trigger).toHaveFocus();
  expect(trigger).toHaveTextContent("Tiếng Việt");
  await waitFor(() => expect(patches.at(-1)?.title).toBe("Tiếng Việt"), {
    timeout: 3000,
  });
});

it("uses the pinned title and section marquee geometry on the actual builder", async () => {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
    width: 600,
  } as DOMRect);
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(200);
  const { container } = renderBuilder();
  const title = await screen.findByRole("button", { name: "Tên đề thi" });
  const titleMarquee = title.querySelector<HTMLElement>(".qz-marquee")!;
  expect(titleMarquee.style.maskImage).toContain("16px");
  expect(
    titleMarquee.querySelector<HTMLElement>('.qz-marquee-track [aria-hidden="true"]')!
      .style.paddingLeft,
  ).toBe("48px");
  const section = container.querySelector<HTMLElement>(
    "[data-outline-section] .qz-marquee",
  )!;
  expect(section.style.maskImage).toContain("10px");
  expect(
    section.querySelector<HTMLElement>('.qz-marquee-track [aria-hidden="true"]')!.style
      .paddingLeft,
  ).toBe("28px");
});
