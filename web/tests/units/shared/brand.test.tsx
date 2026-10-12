import { render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { BrandMark } from "@/components/shared/Brand";

describe("BrandMark under the kit's floor", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("warns in development when nothing names a decided departure", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(<BrandMark height={18} label={false} />);
    expect(warn).toHaveBeenCalledWith(expect.stringContaining("(B-05)"));
  });

  it("stays quiet when the call names its gap", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(<BrandMark height={18} label={false} belowFloor="DG-123" />);
    expect(warn).not.toHaveBeenCalled();
  });

  it("stays quiet at the floor", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(<BrandMark height={24} label={false} />);
    expect(warn).not.toHaveBeenCalled();
  });
});
