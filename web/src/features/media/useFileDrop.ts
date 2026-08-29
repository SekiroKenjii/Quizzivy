import { useEffect, useRef, useState } from "react";

/**
 * Accepts files dropped anywhere on the window.
 *
 * On the window rather than on an element: the design deck puts "Tải lên" in
 * the page header and shows no standing drop card, so there is nothing smaller
 * to aim at. It also keeps drag-and-drop the enhancement it is -- the keyboard
 * and screen-reader path is the header button, so the drop target needs no role
 * of its own.
 *
 * dragover must call preventDefault to signal a drop is allowed, so neither
 * listener can be passive. drop must call it too, unconditionally: having
 * allowed the drop, letting the browser fall back to its default action means
 * navigating to the dropped file and tearing down the SPA. A folder is the
 * everyday way to arrive here -- `types` says "Files" but `files` is empty,
 * because directories surface through the items list instead.
 *
 * The caller receives everything that was dropped and decides what to do with
 * an empty or multi-file drop; a drop that resolves to nothing is still a drop
 * the teacher made, and silence looks like a bug.
 *
 * The handler is held in a ref so the three window listeners are registered
 * once for the life of the hook rather than on every render of the caller.
 */
export function useFileDrop(onFiles: (files: File[]) => void): boolean {
  const [dragging, setDragging] = useState(false);
  const latest = useRef(onFiles);

  useEffect(() => {
    latest.current = onFiles;
  }, [onFiles]);

  useEffect(() => {
    const over = (event: DragEvent) => {
      if (!event.dataTransfer?.types.includes("Files")) return;
      event.preventDefault();
      setDragging(true);
    };
    const leave = (event: DragEvent) => {
      // relatedTarget is null when the pointer leaves the window entirely.
      if (event.relatedTarget === null) setDragging(false);
    };
    const drop = (event: DragEvent) => {
      if (!event.dataTransfer?.types.includes("Files")) return;
      event.preventDefault();
      setDragging(false);
      latest.current([...event.dataTransfer.files]);
    };

    window.addEventListener("dragover", over);
    window.addEventListener("dragleave", leave);
    window.addEventListener("drop", drop);
    return () => {
      window.removeEventListener("dragover", over);
      window.removeEventListener("dragleave", leave);
      window.removeEventListener("drop", drop);
    };
  }, []);

  return dragging;
}
