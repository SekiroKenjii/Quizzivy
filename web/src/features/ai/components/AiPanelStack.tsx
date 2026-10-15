import { lazy, Suspense, useState } from "react";
import type { AiPanelRequest } from "./AiPanel";

const AiPanel = lazy(() => import("./AiPanel"));

/**
 * AiPanelStack draws the AI panels of one page. `open` names the panel that
 * is showing, or null for none, and `requestFor` gives the request of a
 * panel. A panel stays mounted from the first time it opens, drawn closed
 * when another is showing or none is, so its session outlives the drawer and
 * ends with the page. A panel whose run finishes while it is closed asks for
 * itself back through `onOpen`.
 */
export function AiPanelStack<K extends string>({
  open,
  requestFor,
  onOpen,
  onClose,
}: Readonly<{
  open: K | null;
  requestFor: (kind: K) => AiPanelRequest;
  onOpen: (kind: K) => void;
  onClose: () => void;
}>) {
  const [kept, setKept] = useState<readonly K[]>([]);
  const shown = open === null || kept.includes(open) ? kept : [...kept, open];
  if (shown !== kept) setKept(shown);
  return (
    <>
      {shown.map((kind) => (
        <Suspense key={kind} fallback={null}>
          <AiPanel
            request={requestFor(kind)}
            open={open === kind}
            aside={open !== null && open !== kind}
            onClose={onClose}
            onReopen={() => onOpen(kind)}
          />
        </Suspense>
      ))}
    </>
  );
}
