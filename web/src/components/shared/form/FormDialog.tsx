import {
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
  type SubmitEvent,
  type RefObject,
} from "react";
import { useTranslation } from "react-i18next";
import { TriangleAlert, type LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "./DialogShell";
import { FieldControl } from "./fields/FieldControl";
import { fieldErrors, optionalField, visibleFields } from "./fields/rules";
import type { FormField, FormValues } from "./fields/types";
export type {
  FormField,
  FormOption,
  FormInfoRow,
  FormValue,
  FormValues,
} from "./fields/types";

/** FormDialogProps describes a controlled typed form whose caller owns submission and dismissal. */
export type FormDialogProps<V extends FormValues> = Readonly<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string | ((values: V) => string);
  description?: ReactNode | ((values: V) => ReactNode);
  icon?: LucideIcon | undefined;
  danger?: boolean;
  width?: number | undefined;
  className?: string | undefined;
  initial: V;
  fields: readonly FormField<V>[];
  validate?: (values: V) => Partial<Record<keyof V & string, string>> | null;
  submitLabel: string | ((values: V) => string);
  cancelLabel?: string | undefined;
  hideCancel?: boolean;
  pending?: boolean;
  disabled?: boolean;
  error?: string | null;
  onSubmit: (values: V) => void;
  children?: ReactNode;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>;

/** FormDialog edits typed values while open and unlocked, revealing visible errors after submission. */
export function FormDialog<V extends FormValues>({
  open,
  onOpenChange,
  title,
  description,
  icon,
  danger = false,
  width,
  className,
  initial,
  fields,
  validate,
  submitLabel,
  cancelLabel,
  hideCancel = false,
  pending = false,
  disabled = false,
  error = null,
  onSubmit,
  children,
  returnFocus,
}: FormDialogProps<V>) {
  const acceptsChanges = useRef(open && !pending && !disabled);
  useLayoutEffect(() => {
    acceptsChanges.current = open && !pending && !disabled;
    return () => {
      acceptsChanges.current = false;
    };
  }, [open, pending, disabled]);
  const { t } = useTranslation();
  const id = useId();
  const form = useRef<HTMLFormElement>(null);
  const [state, setState] = useState({
    open,
    values: initial,
    touched: false,
    epoch: 0,
  });
  if (state.open !== open)
    setState({
      open,
      values: open ? initial : state.values,
      touched: false,
      epoch: state.epoch + (open ? 1 : 0),
    });
  const values = state.values;
  const shown = visibleFields(fields, values);
  const errors = fieldErrors(
    shown,
    values,
    t("formDialog.required"),
    validate?.(values) ?? null,
  );
  const valid = Object.keys(errors).length === 0;
  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || disabled) return;
    if (!valid) {
      setState((previous) => ({ ...previous, touched: true }));
      const first = shown.find((field) => errors[field.name]);
      if (first) {
        const block = Array.from(
          form.current?.querySelectorAll<HTMLElement>("[data-field-name]") ?? [],
        ).find((node) => node.dataset.fieldName === first.name);
        block
          ?.querySelector<HTMLElement>(
            "input:not([type='hidden']):not([aria-hidden='true']):not([hidden]),textarea,button,[tabindex='0']",
          )
          ?.focus();
      }
      return;
    }
    onSubmit(values);
  }
  const body = shown.length > 0 || children !== undefined || error !== null;
  return (
    <DialogShell
      open={open}
      onOpenChange={onOpenChange}
      width={width}
      className={className}
      returnFocus={returnFocus}
    >
      <DialogShellHeader
        title={typeof title === "function" ? title(values) : title}
        description={
          typeof description === "function" ? description(values) : description
        }
        icon={icon ?? (danger ? TriangleAlert : undefined)}
        tone={danger ? "danger" : "neutral"}
      />
      {body && (
        <DialogShellBody>
          <form
            ref={form}
            id={id}
            noValidate
            onSubmit={submit}
            className="flex flex-col gap-3.5"
          >
            <fieldset disabled={pending || disabled} className="contents">
              {shown.map((field) => {
                const fieldId = `${id}-${field.name}`;
                const message = state.touched ? errors[field.name] : undefined;
                const describedBy =
                  [
                    field.hint ? `${fieldId}-hint` : null,
                    message ? `${fieldId}-error` : null,
                  ]
                    .filter(Boolean)
                    .join(" ") || undefined;
                return (
                  <div
                    key={`${state.epoch}-${field.name}`}
                    data-field-name={field.name}
                    aria-invalid={message !== undefined || undefined}
                    className="flex flex-col gap-1.5"
                  >
                    {field.label && field.kind !== "toggle" && (
                      <label htmlFor={fieldId} className="text-meta font-medium">
                        {field.label}
                        {optionalField(field) && (
                          <span className="text-muted-fg font-normal">
                            {" "}
                            {t("formDialog.optional")}
                          </span>
                        )}
                      </label>
                    )}
                    <FieldControl
                      field={field}
                      values={values}
                      id={fieldId}
                      invalid={message !== undefined}
                      describedBy={describedBy}
                      disabled={!open || pending || disabled}
                      onChange={(next) => {
                        if (!acceptsChanges.current) return;
                        setState((previous) => ({
                          ...previous,
                          values: { ...previous.values, [field.name]: next },
                        }));
                      }}
                    />
                    {field.hint && (
                      <p
                        id={`${fieldId}-hint`}
                        className="text-muted-fg text-xs leading-[1.45]"
                      >
                        {field.hint}
                      </p>
                    )}
                    {message && (
                      <p id={`${fieldId}-error`} className="text-danger-ink text-xs">
                        {message}
                      </p>
                    )}
                  </div>
                );
              })}
            </fieldset>
          </form>
          {children}
          {error !== null && (
            <p role="alert" className="text-danger-ink text-xs">
              {error}
            </p>
          )}
        </DialogShellBody>
      )}
      {!body && <form id={id} noValidate onSubmit={submit} />}
      <DialogShellFooter>
        {!hideCancel && (
          <Button
            type="button"
            variant="outline"
            className="bg-card text-ui h-9 px-3.5 leading-normal shadow-none in-data-[scale=deck]:h-9 in-data-[scale=deck]:px-3.5"
            onClick={() => onOpenChange(false)}
          >
            {cancelLabel ?? t("common.cancel")}
          </Button>
        )}
        <Button
          type="submit"
          form={id}
          variant={danger ? "destructive" : "default"}
          disabled={pending || disabled}
          aria-disabled={!valid || undefined}
          className={cn(
            "text-ui h-9 px-4 leading-normal shadow-none in-data-[scale=deck]:h-9 in-data-[scale=deck]:px-4",
            danger && "bg-danger text-primary-fg hover:bg-danger/90",
            !valid && "opacity-50",
          )}
        >
          {typeof submitLabel === "function" ? submitLabel(values) : submitLabel}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}
