import { useOutletContext } from "react-router";

/** What a page under StudentDetailLayout may set: its own title, once it knows it. */
export interface DetailShell {
  setTitle: (title: string | null) => void;
}

export function useDetailShell(): DetailShell {
  return useOutletContext<DetailShell>();
}
