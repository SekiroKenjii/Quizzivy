import type { Ref } from "react";
import { useTranslation } from "react-i18next";
import { Menu, Moon, PanelLeft, Search, Sun } from "lucide-react";
import { Kbd } from "@/components/ui/kbd";
import { commandKeyLabel } from "@/features/search/useCommandPalette";
import { Breadcrumbs } from "@/layouts/shell/Breadcrumbs";
import type { PageCrumb } from "@/layouts/shell/crumbs";
import { SIDEBAR_ID } from "@/layouts/shell/Sidebar";
import { useResolvedTheme, writeThemePreference } from "@/lib/theme";

const KEY_CAP =
  "bg-muted text-muted-fg in-data-[scale=deck]:text-2xs h-auto min-w-0 border-b leading-3.5";

/**
 * TopBar is the teacher shell's 56px bar: the sidebar toggle, the breadcrumb
 * trail, the search button that opens the command palette, and the switch
 * between the light and dark themes. `drawer` says the sidebar is the drawer
 * below 768px, where the toggle opens it; from 768px the toggle collapses and
 * expands the sidebar, and `expanded` is the state it reports either way.
 */
export function TopBar({
  drawer,
  expanded,
  onToggle,
  toggleRef,
  trail,
  onSearch,
}: Readonly<{
  drawer: boolean;
  expanded: boolean;
  onToggle: () => void;
  toggleRef: Ref<HTMLButtonElement>;
  trail: readonly PageCrumb[];
  onSearch: () => void;
}>) {
  const { t } = useTranslation();
  const theme = useResolvedTheme();
  const dark = theme === "dark";
  const ToggleIcon = drawer ? Menu : PanelLeft;
  const ThemeIcon = dark ? Sun : Moon;
  const collapseOrExpand = expanded
    ? t("teacherShell.sidebar.collapse")
    : t("teacherShell.sidebar.expand");

  return (
    <header className="bg-bg flex h-14 flex-none items-center gap-2 border-b px-4">
      <button
        ref={toggleRef}
        type="button"
        onClick={onToggle}
        aria-label={drawer ? t("teacherShell.sidebar.open") : collapseOrExpand}
        aria-expanded={expanded}
        aria-controls={SIDEBAR_ID}
        className="hover:bg-hover text-fg rounded-seg grid size-8 flex-none place-items-center"
      >
        <ToggleIcon className="size-[1.0625rem]" aria-hidden="true" />
      </button>
      <span aria-hidden="true" className="bg-border mx-1 h-4.5 w-px flex-none" />
      <Breadcrumbs trail={trail} />
      <div className="ml-auto flex flex-none items-center gap-1.5">
        <button
          type="button"
          onClick={onSearch}
          aria-label={t("teacherShell.search")}
          className="bg-card text-muted-fg hover:bg-muted flex h-8.5 w-8.5 flex-none items-center gap-2 rounded-md border pr-2 pl-2.5 text-sm min-[768px]:w-50 min-[1100px]:w-65"
        >
          <Search className="size-[0.9375rem] flex-none" aria-hidden="true" />
          <span className="hidden min-w-0 flex-1 truncate text-left leading-4 min-[768px]:block">
            {t("teacherShell.search")}
          </span>
          <span className="hidden flex-none gap-0.5 min-[768px]:flex">
            <Kbd className={KEY_CAP}>{commandKeyLabel()}</Kbd>
            <Kbd className={KEY_CAP}>{t("palette.keyK")}</Kbd>
          </span>
        </button>
        <button
          type="button"
          onClick={() => writeThemePreference(dark ? "light" : "dark")}
          aria-label={dark ? t("common.lightMode") : t("common.darkMode")}
          className="hover:bg-hover grid size-8.5 flex-none place-items-center rounded-md"
        >
          <ThemeIcon className="size-[1.0625rem]" aria-hidden="true" />
        </button>
      </div>
    </header>
  );
}
