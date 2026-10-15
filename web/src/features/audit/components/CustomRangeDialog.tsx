import { useTranslation } from "react-i18next";
import { Calendar } from "lucide-react";
import { FormDialog, type FormField } from "@/components/shared/form/FormDialog";

type RangeValues = { from: string; to: string };

/**
 * CustomRangeDialog asks for the two days of a custom range, both required
 * and the second not before the first. It opens on `initial`.
 */
export function CustomRangeDialog({
  open,
  initial,
  onOpenChange,
  onApply,
}: Readonly<{
  open: boolean;
  initial: RangeValues;
  onOpenChange: (open: boolean) => void;
  onApply: (range: RangeValues) => void;
}>) {
  const { t } = useTranslation();
  const fields: readonly FormField<RangeValues>[] = [
    { kind: "date", name: "from", label: t("audit.custom.from"), required: true },
    { kind: "date", name: "to", label: t("audit.custom.to"), required: true },
  ];
  return (
    <FormDialog<RangeValues>
      open={open}
      onOpenChange={onOpenChange}
      title={t("audit.custom.title")}
      icon={Calendar}
      initial={initial}
      fields={fields}
      validate={({ from, to }) =>
        from !== "" && to !== "" && from > to ? { to: t("audit.custom.order") } : null
      }
      submitLabel={t("audit.custom.apply")}
      onSubmit={(values) => {
        onApply(values);
        onOpenChange(false);
      }}
    />
  );
}
