import { useEffect, useRef, useState } from "react";

/** useFileDrop accepts window file drops while enabled and clears drag state when disabled. */
export function useFileDrop(onFiles: (files: File[]) => void, enabled = true): boolean {
  const [state, setState] = useState({ enabled, dragging: false });
  if (state.enabled !== enabled) setState({ enabled, dragging: false });
  const latest = useRef(onFiles);

  useEffect(() => {
    latest.current = onFiles;
  }, [onFiles]);

  useEffect(() => {
    if (!enabled) return;
    const setDragging = (dragging: boolean) => setState({ enabled, dragging });
    const over = (event: DragEvent) => {
      if (!event.dataTransfer?.types.includes("Files")) return;
      event.preventDefault();
      setDragging(true);
    };
    const leave = (event: DragEvent) => {
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
  }, [enabled]);

  return enabled && state.enabled && state.dragging;
}
