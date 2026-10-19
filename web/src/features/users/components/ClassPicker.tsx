import { useState } from "react";
import { useTranslation } from "react-i18next";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { RadioCards } from "@/components/shared/form/RadioCard";
import { SearchInput } from "@/components/shared/SearchInput";
import { SEARCH_MIN, classChoicesQuery, type ClassChoice } from "@/features/users/api";
import { Button } from "@/components/ui/button";
import { isTooShort, searchOf } from "@/features/users/userFilters";
import { useDebounced } from "@/lib/useDebounced";

const NONE = "none";

function Choices({
  label,
  failed,
  loading,
  onRetry,
  options,
  value,
  none,
  onChange,
}: Readonly<{
  label: string;
  failed: boolean;
  loading: boolean;
  onRetry: () => void;
  options: readonly ClassChoice[];
  value: string;
  none: string | undefined;
  onChange: (id: string) => void;
}>) {
  const { t } = useTranslation();
  if (failed)
    return (
      <div role="alert" className="flex flex-wrap items-center gap-2">
        <p className="text-danger-ink text-xs">{t("users.classPicker.failed")}</p>
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          {t("common.retry")}
        </Button>
      </div>
    );
  if (loading && options.length === 0)
    return (
      <p role="status" className="text-muted-fg text-meta">
        {t("users.classPicker.loading")}
      </p>
    );
  const cards = options.map((one) => ({
    value: one.id,
    label: one.name,
    hint: one.teacher.name,
  }));
  if (cards.length === 0 && none === undefined)
    return loading ? null : (
      <p className="text-muted-fg text-meta">{t("users.classPicker.empty")}</p>
    );
  return (
    <div className="flex max-h-60 flex-col gap-2 overflow-y-auto p-0.5">
      <RadioCards
        label={label}
        value={value}
        onChange={onChange}
        className="grid-cols-1"
        options={none === undefined ? cards : [{ value: NONE, label: none }, ...cards]}
      />
      {cards.length === 0 && !loading ? (
        <p className="text-muted-fg text-meta">{t("users.classPicker.empty")}</p>
      ) : null}
    </div>
  );
}

/**
 * ClassPicker searches the centre's classes that are not archived, by name or
 * teacher, and picks one; it needs `scope.all`. It says so while the classes
 * load, offers Retry when they fail, and says when a search is too short to send. `value` is the chosen class,
 * kept in the list while a search leaves it out. With `none`, the list opens
 * with an option of that name, which picks no class (`null`).
 */
export function ClassPicker({
  label,
  value,
  none,
  onChange,
}: Readonly<{
  label: string;
  value: ClassChoice | null;
  none?: string | undefined;
  onChange: (choice: ClassChoice | null) => void;
}>) {
  const { t } = useTranslation();
  const [typed, setTyped] = useState("");
  const q = searchOf(useDebounced(typed, 300));
  const classes = useQuery({
    ...classChoicesQuery(q),
    placeholderData: keepPreviousData,
  });
  const found = classes.data ?? [];
  const options =
    value === null || found.some((one) => one.id === value.id)
      ? found
      : [value, ...found];
  const byId = new Map(options.map((one) => [one.id, one]));
  return (
    <div className="flex flex-col gap-2">
      <SearchInput
        className="w-full"
        value={typed}
        onChange={setTyped}
        placeholder={t("users.classPicker.search")}
      />
      {isTooShort(typed) ? (
        <p role="status" className="text-muted-fg text-meta">
          {t("users.search.short", { min: SEARCH_MIN })}
        </p>
      ) : null}
      <Choices
        label={label}
        failed={classes.isError}
        loading={classes.isPending}
        onRetry={() => void classes.refetch()}
        options={options}
        value={value?.id ?? (none === undefined ? "" : NONE)}
        none={none}
        onChange={(id) => onChange(byId.get(id) ?? null)}
      />
    </div>
  );
}
