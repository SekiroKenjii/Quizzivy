import { Outlet, useMatches, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";

interface TitleHandle {
  titleKey: string;
}

/**
 * The deck's S-10 detail shell: a back arrow and the screen's name, in place of
 * the nav bar.
 *
 * A detail screen inside /app is somewhere the student navigated TO, so the
 * tabs would be offering a sideways move they did not ask for. The title comes
 * from the route's `handle`, which keeps it next to the route it names.
 */
export default function StudentDetailLayout() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const matches = useMatches();
  const titled = [...matches].reverse().find((match) => hasTitle(match.handle));

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
        <h1 className="text-sm font-medium">
          {hasTitle(titled?.handle) ? t(titled.handle.titleKey) : t("app.name")}
        </h1>
      </header>
      <main
        className="mx-auto w-full max-w-3xl flex-1 p-4"
        style={{ paddingBottom: "max(1rem, env(safe-area-inset-bottom))" }}
      >
        <Outlet />
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
