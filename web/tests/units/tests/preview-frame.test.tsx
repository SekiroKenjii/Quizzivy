import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { EnginePreview } from "@/features/take-test";
import { StudentPreviewPane } from "@/features/tests/components/StudentPreviewPane";
import { PANEL_FRAME, WINDOW_FRAME } from "@/features/tests/components/previewFrame";
import type { components } from "@/lib/api/schema";
import "@/lib/i18n";

const question: components["schemas"]["StudentQuestion"] = {
  id: "018f0000-0000-7000-8000-0000000000b1",
  sectionId: "018f0000-0000-7000-8000-0000000000c1",
  type: "single_choice",
  prompt: "Choose one.",
  points: 1,
  options: ["went", "go", "gone", "going"].map((text, index) => ({
    id: `018f0000-0000-7000-8000-0000000000d${index}`,
    text,
  })),
};

const previous = Object.getOwnPropertyDescriptor(globalThis, "ResizeObserver");

function frameWidth(width: number) {
  class Observer {
    private readonly callback: ResizeObserverCallback;
    constructor(callback: ResizeObserverCallback) {
      this.callback = callback;
    }
    observe(target: Element) {
      Object.defineProperty(target, "clientWidth", {
        configurable: true,
        value: width,
      });
      this.callback(
        [{ target } as ResizeObserverEntry],
        this as unknown as ResizeObserver,
      );
    }
    unobserve() {}
    disconnect() {}
  }
  Object.defineProperty(globalThis, "ResizeObserver", {
    configurable: true,
    writable: true,
    value: Observer,
  });
}

afterEach(() => {
  if (previous) Object.defineProperty(globalThis, "ResizeObserver", previous);
  else Reflect.deleteProperty(globalThis, "ResizeObserver");
});

function renderFrame(device: "desktop" | "phone") {
  render(
    <StudentPreviewPane device={device} sizes={PANEL_FRAME}>
      <EnginePreview
        title="Unit 5"
        questions={[question, { ...question, id: `${question.id.slice(0, -1)}2` }]}
        sections={[]}
        groups={[]}
      />
    </StudentPreviewPane>,
  );
  return screen.getByRole("radiogroup");
}

describe("the preview frame", () => {
  it("draws each host's two sizes", () => {
    expect(PANEL_FRAME).toEqual({
      desktop: "h-[620px] w-full rounded-xl",
      phone: "h-[720px] w-[390px] rounded-[24px]",
    });
    expect(WINDOW_FRAME).toEqual({
      desktop: "h-full w-[min(1320px,100%)] rounded-xl",
      phone: "h-full w-[390px] rounded-[28px]",
    });
  });

  it("holds the engine in a named region as tall as the frame, which scrolls in its own panes", () => {
    renderFrame("desktop");
    const region = screen.getByRole("region", { name: "Màn hình của học viên" });
    expect(region).not.toHaveAttribute("tabindex");
    expect(region).toHaveClass("relative", "h-full", "flex-col");
    expect(region).not.toHaveClass("overflow-y-auto");
    expect(region.parentElement).toHaveClass("overflow-hidden", "max-w-full", "border");
    expect(region.querySelector("main")).toBeNull();
  });

  it("draws the phone's engine in a phone frame on a wide window", () => {
    frameWidth(388);
    expect(renderFrame("phone")).toHaveClass("grid-cols-2");
    const region = screen.getByRole("region", { name: "Màn hình của học viên" });
    expect(region).toHaveAttribute("data-engine-narrow");
    expect(
      screen.getByRole("button", { name: "Câu 1 · đã làm 0 trên 2" }),
    ).toBeVisible();
    expect(screen.queryByRole("complementary", { name: "Các phần" })).toBeNull();
    expect(screen.queryByText("Câu trả lời ở đây không được lưu.")).toBeNull();
  });

  it("draws the computer's engine in a computer-wide frame", () => {
    frameWidth(1250);
    expect(renderFrame("desktop")).toHaveClass("grid-cols-4");
    const region = screen.getByRole("region", { name: "Màn hình của học viên" });
    expect(region).not.toHaveAttribute("data-engine-narrow");
    expect(screen.getByRole("complementary", { name: "Các phần" })).toBeVisible();
    expect(screen.getByText("Câu trả lời ở đây không được lưu.")).toBeVisible();
  });

  it("marks a computer frame narrower than 1024px, for the engine's lg sizes and its 44px floor", () => {
    frameWidth(1000);
    renderFrame("desktop");
    expect(
      screen.getByRole("region", { name: "Màn hình của học viên" }),
    ).toHaveAttribute("data-engine-narrow");
  });
});
