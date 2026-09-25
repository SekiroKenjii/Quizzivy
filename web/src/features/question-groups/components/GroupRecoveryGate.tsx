import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { ListSkeleton } from "@/components/shared/ListState";
import { openDraftScope, type DraftScope, type LocalDraft } from "@/lib/drafts/store";
import type { StoredGroup } from "../api";
import { readGroupRecovery, type GroupRecovery } from "../recovery";

export type RecoveryState = {
  scope: DraftScope | null;
  draft: LocalDraft | null;
  recovery: GroupRecovery | null;
};
export function GroupRecoveryGate({
  owner,
  stored,
  children,
}: Readonly<{
  owner: string;
  stored: StoredGroup;
  children: (recovery: RecoveryState) => ReactNode;
}>) {
  const { t } = useTranslation();
  const [loaded, setLoaded] = useState<RecoveryState | null>(null);
  const [chosen, setChosen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const id = stored.bundle.group.id;
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const scope = await openDraftScope(owner, `group:${id}`);
        const draft = await scope.read();
        const recovery = draft ? readGroupRecovery(draft.payload, id) : null;
        if (draft && !recovery) await scope.remove();
        if (!cancelled) setLoaded({ scope, draft, recovery });
      } catch {
        if (!cancelled) setLoaded({ scope: null, draft: null, recovery: null });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [owner, id]);
  if (!loaded) return <ListSkeleton rows={5} />;
  if (loaded.recovery && !chosen)
    return (
      <div className="mx-auto flex max-w-2xl flex-col gap-4 py-8">
        <h2 className="text-lg font-semibold">{stored.bundle.group.title}</h2>
        <Alert>
          <AlertTitle>{t("groups.recoveryTitle")}</AlertTitle>
          <AlertDescription>{t("groups.recoveryBody")}</AlertDescription>
        </Alert>
        {error ? <p role="alert">{error}</p> : null}
        <div className="flex flex-wrap gap-2">
          <Button onClick={() => setChosen(true)}>{t("groups.restoreDraft")}</Button>
          <Button
            variant="outline"
            onClick={() => {
              void loaded.scope
                ?.remove()
                .then(() => {
                  setLoaded({ ...loaded, draft: null, recovery: null });
                  setChosen(true);
                })
                .catch(() => setError(t("groups.localUnavailable")));
            }}
          >
            {t("groups.discardDraft")}
          </Button>
        </div>
      </div>
    );
  return <>{children(loaded)}</>;
}
