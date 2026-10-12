import { useEffect, useLayoutEffect, useRef, useState } from "react";

/**
 * useFileDrop accepts window file drops while enabled. Its listeners stay
 * bound for the hook's lifetime and always cancel a drop that carries files,
 * enabled or not, so a file dropped under a dialog never navigates the tab to
 * it; `enabled` gates only the drag state and the callback, and turning it off
 * clears the drag state.
 */
export function useFileDrop(onFiles: (files: File[]) => void, enabled = true): boolean {
  const [state, setState] = useState({ enabled, dragging: false });
  if (state.enabled !== enabled) setState({ enabled, dragging: false });
  const latest = useRef({ onFiles, enabled });

  useLayoutEffect(() => {
    latest.current = { onFiles, enabled };
  }, [onFiles, enabled]);

  useEffect(() => {
    const setDragging = (dragging: boolean) => {
      if (latest.current.enabled) setState({ enabled: true, dragging });
    };
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
      if (!latest.current.enabled) return;
      setDragging(false);
      latest.current.onFiles([...event.dataTransfer.files]);
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

  return enabled && state.enabled && state.dragging;
}
