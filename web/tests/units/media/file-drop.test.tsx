import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { useFileDrop } from "@/features/media/useFileDrop";
import "@/lib/i18n";

/**
 * The window-level drop handler, tested through synthetic events because the
 * failure it guards against — the browser navigating to the dropped file and
 * tearing the SPA down — is a default action, not a rendered state.
 */
function Harness({ onFiles }: { onFiles: (files: File[]) => void }) {
  const dragging = useFileDrop(onFiles);
  return <p>{dragging ? "dragging" : "idle"}</p>;
}

let dropped: File[][] = [];

beforeEach(() => {
  dropped = [];
  render(<Harness onFiles={(files) => dropped.push(files)} />);
});

function transfer(files: File[], types = ["Files"]) {
  return { types, files } as unknown as DataTransfer;
}

function fire(
  type: string,
  dataTransfer: DataTransfer | null,
  relatedTarget: unknown = null,
) {
  const event = new Event(type, { bubbles: true, cancelable: true });
  Object.defineProperty(event, "dataTransfer", { value: dataTransfer });
  Object.defineProperty(event, "relatedTarget", { value: relatedTarget });
  act(() => {
    window.dispatchEvent(event);
  });
  return event;
}

const clip = (name: string) =>
  new File([new Uint8Array(4)], name, { type: "audio/mpeg" });

describe("the window file drop", () => {
  it("prevents the browser's default even when the drop carries no file", () => {
    fire("dragover", transfer([]));
    const event = fire("drop", transfer([]));

    // Without this the browser navigates to the dropped item and the SPA dies.
    expect(event.defaultPrevented).toBe(true);
    expect(dropped).toEqual([[]]);
  });

  it("clears the drag overlay on a drop that yields nothing, not just on a good one", () => {
    fire("dragover", transfer([]));
    expect(screen.getByText("dragging")).toBeInTheDocument();

    fire("drop", transfer([]));

    expect(screen.getByText("idle")).toBeInTheDocument();
  });

  it("hands over every dropped file, so the caller decides about a multi-drop", () => {
    fire("drop", transfer([clip("a.mp3"), clip("b.mp3"), clip("c.mp3")]));

    expect(dropped[0]?.map((file) => file.name)).toEqual(["a.mp3", "b.mp3", "c.mp3"]);
  });

  it("ignores a drag that is not carrying files", () => {
    fire("dragover", transfer([], ["text/plain"]));
    expect(screen.getByText("idle")).toBeInTheDocument();

    const event = fire("drop", transfer([], ["text/plain"]));

    expect(event.defaultPrevented).toBe(false);
    expect(dropped).toEqual([]);
  });

  it("keeps the overlay while the pointer crosses into a child element", () => {
    fire("dragover", transfer([clip("a.mp3")]));
    fire("dragleave", transfer([clip("a.mp3")]), document.body);

    expect(screen.getByText("dragging")).toBeInTheDocument();

    fire("dragleave", transfer([clip("a.mp3")]), null);

    expect(screen.getByText("idle")).toBeInTheDocument();
  });

  it("registers its listeners once, however often the caller re-renders", () => {
    const add = vi.spyOn(window, "addEventListener");
    const { rerender } = render(<Harness onFiles={() => undefined} />);
    const before = add.mock.calls.length;

    rerender(<Harness onFiles={() => undefined} />);
    rerender(<Harness onFiles={() => undefined} />);

    expect(add.mock.calls.length).toBe(before);
    add.mockRestore();
  });
});
