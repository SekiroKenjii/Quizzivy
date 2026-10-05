import type { ReactNode, RefObject } from "react";
import type { LucideIcon } from "lucide-react";
import { FormDialog } from "@/components/shared/form/FormDialog";

/** ConfirmDialogProps preserves the confirming and notice dialog contracts for existing callers. */
export type ConfirmDialogProps = Readonly<{
  open: boolean;
  className?: string;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: ReactNode;
  children?: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  destructive?: boolean;
  disabled?: boolean;
  pending?: boolean;
  error?: string | null;
  onConfirm?: () => void;
  returnFocus?: RefObject<HTMLElement | null>;
  icon?: LucideIcon;
}>;
const INITIAL = {};
const FIELDS = [] as const;
/** ConfirmDialog asks before a caller-owned action or shows a notice that closes on confirmation. */
export function ConfirmDialog({
  open,
  className,
  onOpenChange,
  title,
  description,
  children,
  confirmLabel,
  cancelLabel,
  destructive = false,
  disabled = false,
  pending = false,
  error = null,
  onConfirm,
  returnFocus,
  icon,
}: ConfirmDialogProps) {
  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      className={className}
      icon={icon}
      danger={destructive}
      initial={INITIAL}
      fields={FIELDS}
      submitLabel={confirmLabel}
      cancelLabel={cancelLabel}
      hideCancel={onConfirm === undefined}
      disabled={disabled}
      pending={pending}
      error={error}
      returnFocus={returnFocus}
      onSubmit={() => (onConfirm === undefined ? onOpenChange(false) : onConfirm())}
    >
      {children}
    </FormDialog>
  );
}
