import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { StudentPreview } from "@/features/tests/components/StudentPreview";
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
      <StudentPreview questions={[question]} />
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

  it("scrolls its content in a named region a keyboard reaches", () => {
    renderFrame("desktop");
    const region = screen.getByRole("region", { name: "Màn hình của học viên" });
    expect(region).toHaveAttribute("tabindex", "0");
    expect(region).toHaveClass("overflow-y-auto", "relative");
    expect(region.parentElement).toHaveClass("overflow-hidden", "max-w-full", "border");
  });

  it("lays the paper out by the frame's width, not the window's", () => {
    frameWidth(388);
    expect(renderFrame("phone")).toHaveClass("grid-cols-2");
  });

  it("lays a computer-wide frame out as wide", () => {
    frameWidth(1250);
    expect(renderFrame("desktop")).toHaveClass("grid-cols-4");
  });
});
