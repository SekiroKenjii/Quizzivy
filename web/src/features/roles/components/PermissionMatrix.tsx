import {
  Fragment,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type Ref,
} from "react";
import { useTranslation } from "react-i18next";
import { Checkbox } from "@/components/ui/checkbox";
import type { PermissionKey } from "@/features/auth/permissions";
import type { CatalogueEntry, RoleSummary } from "@/features/roles/api";
import {
  isOn,
  lockOf,
  grantsOf,
  type Caller,
  type CellLock,
  type Draft,
} from "@/features/roles/cells";
import { ColumnHighlight } from "@/features/roles/components/ColumnHighlight";
import { copyOf, type PermissionCopy } from "@/features/roles/permissionLabels";
import { useColumnRect } from "@/features/roles/useColumnRect";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { roleName } from "@/lib/roleName";
import { cn } from "@/lib/utils";

const REASONS: Readonly<Record<CellLock, string>> = {
  admin: "roles.lock.admin",
  student: "roles.lock.student",
  self: "roles.lock.self",
  subset: "roles.lock.subset",
};

const GROUPS: Readonly<Record<CatalogueEntry["group"], string>> = {
  content: "roles.group.content",
  teaching: "roles.group.teaching",
  people: "roles.group.people",
  system: "roles.group.system",
};

const WIDE_LABEL_REM = 15;
const NARROW_LABEL_REM = 10;
const ROLE_REM = 6.5;
const PAD_REM = 1;
const GAP_REM = 0.5;

const STUCK = "sticky left-0 z-3 -ml-4 pl-4";

const CELL =
  "size-4.5 rounded-[0.3125rem] in-data-[scale=deck]:rounded-[0.3125rem] in-data-[scale=deck]:checked:after:size-3 aria-disabled:cursor-not-allowed aria-disabled:border-ring/50 aria-disabled:checked:bg-primary/50 aria-disabled:checked:border-primary/50 data-[changed=true]:shadow-[0_0_0_2px_var(--card),0_0_0_4px_var(--accent-c)]";

interface Row {
  key: PermissionKey;
  heading: CatalogueEntry["group"] | null;
  copy: PermissionCopy;
}

function position(target: EventTarget): [number, number] | null {
  if (!(target instanceof HTMLElement)) return null;
  const at = target.dataset["pos"]?.split(":").map(Number);
  return at?.length === 2 ? [at[0]!, at[1]!] : null;
}

/**
 * PermissionMatrix is the permission table: a row for each catalogue key,
 * grouped, and a column for each role, with a checkbox where they meet. A
 * cell the caller cannot change stays focusable and answers `aria-disabled`,
 * its colours at half strength, with the reason as its title and its
 * description; a cell that differs from what the server holds is marked.
 * The cells are one tab stop, a grid: the arrows move between them, Home and
 * End to the row's ends, with Control to the table's. A role header
 * highlights its column with a frame (`highlight` is the role id), pressed
 * again it takes the frame off. Wherever the table is wider than its box it
 * scrolls sideways under a label column that stays put, narrower below a
 * content width of 768px, and a focused cell scrolls clear of that column.
 */
export function PermissionMatrix({
  roles,
  catalogue,
  draft,
  caller,
  highlight,
  ref,
  onToggle,
  onHighlight,
}: Readonly<{
  roles: readonly RoleSummary[];
  catalogue: readonly CatalogueEntry[];
  draft: Draft;
  caller: Caller;
  highlight: string | null;
  ref?: Ref<HTMLDivElement>;
  onToggle: (role: RoleSummary, key: PermissionKey) => void;
  onHighlight: (role: RoleSummary) => void;
}>) {
  const { t } = useTranslation();
  const id = useId();
  const roomy = useContentWidthAtLeast(768);
  const label = roomy ? WIDE_LABEL_REM : NARROW_LABEL_REM;
  const grid = useRef<HTMLDivElement>(null);
  const [rectRef, rect] = useColumnRect(highlight);
  const [active, setActive] = useState<[number, number]>([0, 0]);
  const rows = useMemo<Row[]>(() => {
    const known = catalogue.flatMap((entry) => {
      const copy = copyOf(entry.key);
      return copy === null ? [] : [{ key: entry.key, group: entry.group, copy }];
    });
    return known.map((row, index) => ({
      key: row.key,
      copy: row.copy,
      heading: known[index - 1]?.group === row.group ? null : row.group,
    }));
  }, [catalogue]);
  const names = useMemo(() => roles.map((role) => roleName(role, t)), [roles, t]);
  const activeRow = Math.min(active[0], rows.length - 1);
  const activeCol = Math.min(active[1], roles.length - 1);

  function navigate(event: KeyboardEvent<HTMLDivElement>) {
    const from = position(event.target);
    if (from === null) return;
    const last: [number, number] = [rows.length - 1, roles.length - 1];
    let [row, col] = from;
    switch (event.key) {
      case "ArrowUp":
        row -= 1;
        break;
      case "ArrowDown":
        row += 1;
        break;
      case "ArrowLeft":
        col -= 1;
        break;
      case "ArrowRight":
        col += 1;
        break;
      case "Home":
        col = 0;
        if (event.ctrlKey) row = 0;
        break;
      case "End":
        col = last[1];
        if (event.ctrlKey) row = last[0];
        break;
      default:
        return;
    }
    event.preventDefault();
    row = Math.max(0, Math.min(last[0], row));
    col = Math.max(0, Math.min(last[1], col));
    grid.current?.querySelector<HTMLElement>(`[data-pos="${row}:${col}"]`)?.focus();
  }

  function remember(event: { target: EventTarget }) {
    const at = position(event.target);
    if (at !== null) setActive(at);
  }

  const style = {
    gridTemplateColumns: `minmax(${label}rem, 2fr) repeat(${roles.length}, ${ROLE_REM}rem)`,
  };
  return (
    <div
      ref={ref}
      className="bg-card shadow-card scroll-mt-4 overflow-x-auto rounded-xl border"
      style={{ scrollPaddingLeft: `${label + PAD_REM + GAP_REM}rem` }}
    >
      <div
        ref={rectRef}
        className="relative"
        style={{
          minWidth: `${label + 2 * PAD_REM + roles.length * (ROLE_REM + GAP_REM)}rem`,
        }}
      >
        <ColumnHighlight rect={rect} />
        <div
          ref={grid}
          role="grid"
          tabIndex={-1}
          aria-label={t("roles.matrix")}
          onKeyDown={navigate}
          onFocus={remember}
        >
          <div
            role="row"
            style={style}
            className="bg-muted grid h-11 items-center gap-2 px-4 text-[0.78125rem] font-semibold"
          >
            <span
              role="columnheader"
              className={cn(
                STUCK,
                "bg-muted text-muted-fg flex h-11 items-center font-medium",
              )}
            >
              {t("roles.permission")}
            </span>
            {roles.map((role, col) => (
              <span key={role.id} role="columnheader" className="min-w-0">
                <button
                  type="button"
                  data-role-col={role.id}
                  aria-pressed={highlight === role.id}
                  title={names[col]}
                  className="hover:bg-hover block w-full overflow-hidden rounded-md py-1 text-center text-ellipsis whitespace-nowrap"
                  onClick={() => onHighlight(role)}
                >
                  {names[col]}
                </button>
              </span>
            ))}
          </div>
          {rows.map((row, r) => {
            return (
              <Fragment key={row.key}>
                {row.heading !== null && (
                  <div role="row">
                    <div
                      role="rowheader"
                      aria-colspan={roles.length + 1}
                      className="text-muted-fg bg-sidebar border-t px-4 pt-2.5 pb-1.5 text-xs leading-4.5 font-semibold"
                    >
                      <span className="sticky left-4 inline-block">
                        {t(GROUPS[row.heading])}
                      </span>
                    </div>
                  </div>
                )}
                <div
                  role="row"
                  style={style}
                  className="grid min-h-11.5 items-center gap-2 border-t px-4 text-[0.84375rem]"
                >
                  <div role="rowheader" className={cn(STUCK, "bg-card min-w-0 py-1.5")}>
                    <span className="block">{t(row.copy.label)}</span>
                    {row.copy.hint !== undefined && (
                      <span className="text-muted-fg block text-xs">
                        {t(row.copy.hint)}
                      </span>
                    )}
                  </div>
                  {roles.map((role, col) => {
                    const lock = lockOf(role, row.key, caller);
                    const reason = lock === null ? undefined : t(REASONS[lock]);
                    return (
                      <span
                        key={role.id}
                        role="gridcell"
                        className="flex justify-center"
                      >
                        <Checkbox
                          checked={isOn(role, row.key, draft)}
                          data-pos={`${r}:${col}`}
                          data-changed={
                            isOn(role, row.key, draft) !== grantsOf(role).has(row.key)
                          }
                          tabIndex={r === activeRow && col === activeCol ? 0 : -1}
                          aria-label={t("roles.cell", {
                            permission: t(row.copy.label),
                            role: names[col],
                          })}
                          aria-disabled={lock === null ? undefined : true}
                          aria-describedby={lock === null ? undefined : `${id}-${lock}`}
                          title={reason}
                          className={CELL}
                          onChange={() => {
                            if (lock === null) onToggle(role, row.key);
                          }}
                        />
                      </span>
                    );
                  })}
                </div>
              </Fragment>
            );
          })}
        </div>
        {(Object.keys(REASONS) as CellLock[]).map((lock) => (
          <span key={lock} id={`${id}-${lock}`} className="sr-only">
            {t(REASONS[lock])}
          </span>
        ))}
      </div>
    </div>
  );
}
