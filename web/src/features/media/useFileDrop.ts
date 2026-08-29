import { useEffect, useState } from "react";

/**
 * Accepts a file dropped anywhere on the window.
 *
 * On the window rather than on an element: the design deck puts "Tải lên" in
 * the page header and shows no standing drop card, so there is nothing smaller
 * to aim at. It also keeps drag-and-drop the enhancement it is -- the keyboard
 * and screen-reader path is the header button, so the drop target needs no role
 * of its own.
 *
 * dragover must call preventDefault to signal a drop is allowed, so neither
 * listener can be passive.
 */
export function useFileDrop(onFile: (file: File) => void): boolean {
  const [dragging, setDragging] = useState(false);

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
      const file = event.dataTransfer?.files[0];
      if (!file) return;
      event.preventDefault();
      setDragging(false);
      onFile(file);
    };

    window.addEventListener("dragover", over);
    window.addEventListener("dragleave", leave);
    window.addEventListener("drop", drop);
    return () => {
      window.removeEventListener("dragover", over);
      window.removeEventListener("dragleave", leave);
      window.removeEventListener("drop", drop);
    };
  }, [onFile]);

  return dragging;
}
