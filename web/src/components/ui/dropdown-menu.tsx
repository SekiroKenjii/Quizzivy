import type * as React from "react";
import { DropdownMenu as Primitive } from "radix-ui";

import { Check } from "lucide-react";

import { cn } from "@/lib/utils";

function DropdownMenu(props: React.ComponentProps<typeof Primitive.Root>) {
  return <Primitive.Root data-slot="dropdown-menu" {...props} />;
}

function DropdownMenuTrigger(props: React.ComponentProps<typeof Primitive.Trigger>) {
  return <Primitive.Trigger data-slot="dropdown-menu-trigger" {...props} />;
}

/**
 * DropdownMenuContent is the menu's surface, rendered in a portal: the deck's
 * container, with the card fill, a 10px radius, the float shadow, 5px of
 * padding and a height capped at 60% of the viewport and at the room beside
 * its trigger, past which it scrolls. It keeps 8px clear of the window's
 * edges, flipping or shifting before it would come closer, unless
 * `collisionPadding` says otherwise. It sets no width.
 */
function DropdownMenuContent({
  className,
  sideOffset = 4,
  collisionPadding,
  ...props
}: React.ComponentProps<typeof Primitive.Content>) {
  return (
    <Primitive.Portal>
      <Primitive.Content
        data-slot="dropdown-menu-content"
        sideOffset={sideOffset}
        collisionPadding={collisionPadding ?? 8}
        className={cn(
          "text-popover-foreground min-w-40 overflow-hidden border",
          "data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0",
          "bg-card shadow-float z-(--z-popover) max-h-[min(60vh,var(--radix-dropdown-menu-content-available-height))] overflow-y-auto rounded-lg p-1.25",
          className,
        )}
        {...props}
      />
    </Primitive.Portal>
  );
}

/**
 * DropdownMenuLabel is a menu's title row, above its first item. It is not an
 * item: arrow keys and type-ahead pass it by.
 */
function DropdownMenuLabel({
  className,
  ...props
}: React.ComponentProps<typeof Primitive.Label>) {
  return (
    <Primitive.Label
      data-slot="dropdown-menu-label"
      className={cn(
        "px-2 py-1.5",
        "text-muted-fg text-caption pt-1 pb-1.5 leading-normal",
        className,
      )}
      {...props}
    />
  );
}

/**
 * DropdownMenuItem is one action of a menu; `variant="destructive"` draws it
 * in the danger ink. It is the deck's row: 13px text, a 14px icon, 9px between
 * its parts, and 45% opacity when disabled.
 */
function DropdownMenuItem({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<typeof Primitive.Item> & {
  variant?: "default" | "destructive";
}) {
  return (
    <Primitive.Item
      data-slot="dropdown-menu-item"
      data-variant={variant}
      className={cn(
        "focus:bg-accent focus:text-accent-foreground relative flex cursor-default items-center rounded-sm px-2 text-sm outline-none select-none",
        "data-[disabled]:pointer-events-none",
        "[&_svg]:shrink-0",
        "gap-2.25 py-1.75 leading-4 data-[disabled]:opacity-45 [&_svg:not([class*='size-'])]:size-3.5",
        "data-[variant=destructive]:text-danger-ink data-[variant=destructive]:focus:bg-hover data-[variant=destructive]:focus:text-danger-ink",
        className,
      )}
      {...props}
    />
  );
}

/**
 * DropdownMenuCheckboxItem is a menu row that toggles and leaves the menu
 * open, so several can be picked without reopening it. The tick sits in the
 * deck's 16px box, which fills with the primary colour when the row is
 * checked.
 */
function DropdownMenuCheckboxItem({
  className,
  children,
  ...props
}: React.ComponentProps<typeof Primitive.CheckboxItem>) {
  return (
    <Primitive.CheckboxItem
      data-slot="dropdown-menu-checkbox-item"
      onSelect={(event) => event.preventDefault()}
      className={cn(
        "focus:bg-accent focus:text-accent-foreground relative flex cursor-default items-center rounded-sm pr-2 text-sm outline-none select-none",
        "data-[disabled]:pointer-events-none",
        "gap-2.25 py-1.75 pl-2 leading-4 data-[disabled]:opacity-45",
        className,
      )}
      {...props}
    >
      <span
        className={cn(
          "left-2 flex items-center justify-center",
          "border-ring bg-card text-primary-fg static size-4 flex-none rounded-[0.25rem] border",
          "in-data-[state=checked]:border-primary in-data-[state=checked]:bg-primary",
        )}
      >
        <Primitive.ItemIndicator>
          <Check className="size-[0.6875rem]" aria-hidden="true" />
        </Primitive.ItemIndicator>
      </span>
      {children}
    </Primitive.CheckboxItem>
  );
}

function DropdownMenuRadioGroup(
  props: React.ComponentProps<typeof Primitive.RadioGroup>,
) {
  return <Primitive.RadioGroup data-slot="dropdown-menu-radio-group" {...props} />;
}

/**
 * DropdownMenuRadioItem is one choice of a DropdownMenuRadioGroup: the menu
 * closes on it, and the chosen row carries the same tick the checkbox item
 * draws, in the deck's 16px box.
 */
function DropdownMenuRadioItem({
  className,
  children,
  ...props
}: React.ComponentProps<typeof Primitive.RadioItem>) {
  return (
    <Primitive.RadioItem
      data-slot="dropdown-menu-radio-item"
      className={cn(
        "focus:bg-accent focus:text-accent-foreground relative flex cursor-default items-center rounded-sm pr-2 text-sm outline-none select-none",
        "data-[disabled]:pointer-events-none",
        "gap-2.25 py-1.75 pl-2 leading-4 data-[disabled]:opacity-45",
        className,
      )}
      {...props}
    >
      <span
        className={cn(
          "left-2 flex items-center justify-center",
          "border-ring bg-card text-primary-fg static size-4 flex-none rounded-[0.25rem] border",
          "in-data-[state=checked]:border-primary in-data-[state=checked]:bg-primary",
        )}
      >
        <Primitive.ItemIndicator>
          <Check className="size-[0.6875rem]" aria-hidden="true" />
        </Primitive.ItemIndicator>
      </span>
      {children}
    </Primitive.RadioItem>
  );
}

/**
 * DropdownMenuItemText is an item's label when it may be longer than the
 * menu is wide: it takes the room the row has left and ends in an ellipsis
 * instead of wrapping.
 */
function DropdownMenuItemText({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="dropdown-menu-item-text"
      className={cn("min-w-0 flex-1 truncate", className)}
      {...props}
    />
  );
}

/**
 * DropdownMenuMeta is the short figure or note at the end of a menu row,
 * muted and in tabular figures.
 */
function DropdownMenuMeta({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="dropdown-menu-meta"
      className={cn(
        "text-muted-fg ml-auto flex-none text-xs leading-[0.9375rem] tabular-nums",
        className,
      )}
      {...props}
    />
  );
}

/**
 * DropdownMenuSeparator is the line between two groups of a menu, as wide as
 * the rows, with no space above or below.
 */
function DropdownMenuSeparator({
  className,
  ...props
}: React.ComponentProps<typeof Primitive.Separator>) {
  return (
    <Primitive.Separator
      data-slot="dropdown-menu-separator"
      className={cn("bg-border h-px", "mx-0 my-0", className)}
      {...props}
    />
  );
}

export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuCheckboxItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuItemText,
  DropdownMenuMeta,
  DropdownMenuSeparator,
};
