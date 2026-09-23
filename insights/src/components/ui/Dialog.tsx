import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import type { ComponentProps } from "react";
import { cn } from "../../lib/cn";

const Dialog = DialogPrimitive.Root;
const DialogTrigger = DialogPrimitive.Trigger;
const DialogClose = DialogPrimitive.Close;

function DialogContent({ className, children, ...props }: ComponentProps<typeof DialogPrimitive.Content>) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-slate-950/45 backdrop-blur-[2px]" />
      <DialogPrimitive.Content
        className={cn(
          "fixed left-1/2 top-1/2 z-50 max-h-[calc(100vh-48px)] w-[calc(100vw-48px)] max-w-4xl -translate-x-1/2 -translate-y-1/2 overflow-auto rounded-lg border border-[var(--border)] bg-[var(--surface-raised)] p-6 text-[var(--ink)] shadow-[var(--shadow-overlay)] focus:outline-none",
          className,
        )}
        {...props}
      >
        {children}
        <DialogPrimitive.Close className="absolute right-4 top-4 grid size-8 place-items-center rounded-md text-[var(--muted)] transition-colors hover:bg-[var(--surface-subtle)] hover:text-[var(--ink)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]" aria-label="关闭">
          <X aria-hidden="true" />
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

function DialogHeader({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("grid gap-1.5 pr-10", className)} {...props} />;
}
function DialogFooter({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("mt-6 flex justify-end gap-2", className)} {...props} />;
}
function DialogTitle({ className, ...props }: ComponentProps<typeof DialogPrimitive.Title>) {
  return <DialogPrimitive.Title className={cn("m-0 text-lg font-semibold", className)} {...props} />;
}
function DialogDescription({ className, ...props }: ComponentProps<typeof DialogPrimitive.Description>) {
  return <DialogPrimitive.Description className={cn("m-0 text-sm text-[var(--muted)]", className)} {...props} />;
}

export { Dialog, DialogTrigger, DialogClose, DialogContent, DialogHeader, DialogFooter, DialogTitle, DialogDescription };
