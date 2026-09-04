import { useState } from "react";
import { Outlet, useMatches, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { DetailShell } from "@/layouts/detailShell";

interface TitleHandle {
  titleKey: string;
}

/**
 * The deck's S-10 detail shell: a back arrow and the screen's name, in place of
 * the nav bar.
 */
export default function StudentDetailLayout() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const matches = useMatches();
  const titled = [...matches].reverse().find((match) => hasTitle(match.handle));
  const [own, setTitle] = useState<string | null>(null);

  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex h-14 items-center gap-2 border-b px-4">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("common.back")}
          onClick={() => void navigate(-1)}
        >
          <ArrowLeft aria-hidden="true" />
        </Button>
        <h1 className="truncate text-sm font-medium">
          {own ??
            (hasTitle(titled?.handle) ? t(titled.handle.titleKey) : t("app.name"))}
        </h1>
      </header>
      <main
        className="mx-auto w-full max-w-3xl flex-1 p-4"
        style={{ paddingBottom: "max(1rem, env(safe-area-inset-bottom))" }}
      >
        <Outlet context={{ setTitle } satisfies DetailShell} />
      </main>
    </div>
  );
}

function hasTitle(handle: unknown): handle is TitleHandle {
  return (
    typeof handle === "object" &&
    handle !== null &&
    typeof (handle as TitleHandle).titleKey === "string"
  );
}
