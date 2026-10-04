import type * as React from "react";
import { DropdownMenu as Primitive } from "radix-ui";

import { Check } from "lucide-react";

import { useDeckScale } from "@/components/ui/deck-scale";
import { cn } from "@/lib/utils";

function DropdownMenu(props: React.ComponentProps<typeof Primitive.Root>) {
  return <Primitive.Root data-slot="dropdown-menu" {...props} />;
}

function DropdownMenuTrigger(props: React.ComponentProps<typeof Primitive.Trigger>) {
  return <Primitive.Trigger data-slot="dropdown-menu-trigger" {...props} />;
}

/**
 * DropdownMenuContent is the menu's surface, rendered in a portal. On a deck
 * surface it carries `data-scale="deck"` itself and is the deck's container:
 * the card fill, a 10px radius, the float shadow, 5px of padding and a height
 * capped at 60% of the viewport, past which it scrolls. It sets no width. A
 * caller overrides this geometry with a `data-[scale=deck]:` class, because
 * the content has no deck ancestor in the DOM.
 */
function DropdownMenuContent({
  className,
  sideOffset = 4,
  ...props
}: React.ComponentProps<typeof Primitive.Content>) {
  const deck = useDeckScale();
  return (
    <Primitive.Portal>
      <Primitive.Content
        data-slot="dropdown-menu-content"
        data-scale={deck ? "deck" : undefined}
        sideOffset={sideOffset}
        className={cn(
          "bg-popover text-popover-foreground z-50 min-w-40 overflow-hidden rounded-md border p-1 shadow-md",
          "data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0",
          "data-[scale=deck]:bg-card data-[scale=deck]:shadow-float data-[scale=deck]:z-(--z-popover) data-[scale=deck]:max-h-[60vh] data-[scale=deck]:overflow-y-auto data-[scale=deck]:rounded-lg data-[scale=deck]:p-1.25",
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
        "text-muted-foreground px-2 py-1.5 text-xs",
        "in-data-[scale=deck]:text-muted-fg in-data-[scale=deck]:text-caption in-data-[scale=deck]:pt-1 in-data-[scale=deck]:pb-1.5 in-data-[scale=deck]:leading-normal",
        className,
      )}
      {...props}
    />
  );
}

/**
 * DropdownMenuItem is one action of a menu; `variant="destructive"` draws it
 * in the danger ink. On a deck surface it is the deck's row: 13px text, a 14px
 * icon, 9px between its parts, and 45% opacity when disabled.
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
        "focus:bg-accent focus:text-accent-foreground relative flex cursor-default items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none select-none",
        "data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
        "data-[variant=destructive]:text-destructive-ink data-[variant=destructive]:focus:bg-destructive/10",
        "[&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        "in-data-[scale=deck]:gap-2.25 in-data-[scale=deck]:py-1.75 in-data-[scale=deck]:leading-[1.265] in-data-[scale=deck]:data-[disabled]:opacity-45 in-data-[scale=deck]:[&_svg:not([class*='size-'])]:size-3.5",
        "in-data-[scale=deck]:data-[variant=destructive]:text-danger-ink in-data-[scale=deck]:data-[variant=destructive]:focus:bg-hover in-data-[scale=deck]:data-[variant=destructive]:focus:text-danger-ink",
        className,
      )}
      {...props}
    />
  );
}

/**
 * DropdownMenuCheckboxItem is a menu row that toggles and leaves the menu
 * open, so several can be picked without reopening it. On a deck surface the
 * tick sits in the deck's 16px box, which fills with the primary colour when
 * the row is checked.
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
        "focus:bg-accent focus:text-accent-foreground relative flex cursor-default items-center gap-2 rounded-sm py-1.5 pr-2 pl-7 text-sm outline-none select-none",
        "data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
        "in-data-[scale=deck]:gap-2.25 in-data-[scale=deck]:py-1.75 in-data-[scale=deck]:pl-2 in-data-[scale=deck]:leading-[1.265] in-data-[scale=deck]:data-[disabled]:opacity-45",
        className,
      )}
      {...props}
    >
      <span
        className={cn(
          "absolute left-2 flex size-3.5 items-center justify-center",
          "in-data-[scale=deck]:border-ring in-data-[scale=deck]:bg-card in-data-[scale=deck]:text-primary-fg in-data-[scale=deck]:static in-data-[scale=deck]:size-4 in-data-[scale=deck]:flex-none in-data-[scale=deck]:rounded-[0.25rem] in-data-[scale=deck]:border",
          "in-data-[scale=deck]:in-data-[state=checked]:border-primary in-data-[scale=deck]:in-data-[state=checked]:bg-primary",
        )}
      >
        <Primitive.ItemIndicator>
          <Check
            className="size-3.5 in-data-[scale=deck]:size-[0.6875rem]"
            aria-hidden="true"
          />
        </Primitive.ItemIndicator>
      </span>
      {children}
    </Primitive.CheckboxItem>
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
        "text-muted-fg ml-auto flex-none text-xs leading-[1.265] tabular-nums",
        className,
      )}
      {...props}
    />
  );
}

/**
 * DropdownMenuSeparator is the line between two groups of a menu. On a deck
 * surface it is the deck's: as wide as the rows, with no space above or below.
 */
function DropdownMenuSeparator({
  className,
  ...props
}: React.ComponentProps<typeof Primitive.Separator>) {
  return (
    <Primitive.Separator
      data-slot="dropdown-menu-separator"
      className={cn(
        "bg-border -mx-1 my-1 h-px",
        "in-data-[scale=deck]:mx-0 in-data-[scale=deck]:my-0",
        className,
      )}
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
  DropdownMenuItemText,
  DropdownMenuMeta,
  DropdownMenuSeparator,
};
