import { Toaster as Sonner, toast } from "sonner";

/** F-08: bottom-right, one line, four seconds, no colour, no title. */
function Toaster() {
  return (
    <Sonner
      position="bottom-right"
      duration={4000}
      gap={8}
      toastOptions={{
        unstyled: true,
        classNames: {
          toast:
            "bg-popover text-popover-foreground flex w-80 items-center gap-3 rounded-md border px-4 py-3 text-sm shadow-md",
          title: "min-w-0 flex-1 truncate font-normal",
          actionButton:
            "ml-auto shrink-0 text-sm font-medium underline-offset-4 hover:underline",
        },
      }}
    />
  );
}

export { Toaster, toast };
