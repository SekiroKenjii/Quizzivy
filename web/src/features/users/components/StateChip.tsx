import { useTranslation } from "react-i18next";
import { ChipBox } from "@/components/shared/form/ChipInput";
import type { UserStateFilter } from "@/features/users/api";

/**
 * StateChip is the state the list is narrowed to when the Overview links to
 * it, as the deck's 22px tag with a remove button. It is the only filter the
 * list shows as a chip.
 */
export function StateChip({
  state,
  onRemove,
}: Readonly<{ state: UserStateFilter; onRemove: () => void }>) {
  const { t } = useTranslation();
  const label =
    state === "never_signed_in"
      ? t("users.status.neverSignedIn")
      : t("users.status.mustChangePassword");
  return (
    <div role="group" aria-label={t("users.filter.label")}>
      <ChipBox
        values={[label]}
        onRemove={onRemove}
        removeLabel={(value) => t("users.filter.remove", { label: value })}
        frame={false}
      >
        {null}
      </ChipBox>
    </div>
  );
}
