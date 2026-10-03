"use client";

import { useState } from "react";

import { Button } from "@workspace/ui/components/button";
import { ConfirmDialog } from "@workspace/ui/components/confirm-dialog";

type DialogAnswer = "none" | "confirmed" | "cancelled";

const ANSWER_TEXT: Record<DialogAnswer, string> = {
  none: "No answer yet.",
  confirmed: "Last answer: confirmed.",
  cancelled: "Last answer: cancelled.",
};

export function ConfirmDialogDemo() {
  const [lastAnswer, setLastAnswer] = useState<DialogAnswer>("none");

  return (
    <div className="flex flex-wrap items-center gap-3">
      <ConfirmDialog
        trigger={<Button variant="outline">Open confirm dialog</Button>}
        title="Remove example item?"
        description="This is sample text. Nothing is removed when you confirm."
        confirmLabel="Remove"
        confirmVariant="destructive"
        onConfirm={() => setLastAnswer("confirmed")}
        onCancel={() => setLastAnswer("cancelled")}
      />
      <p aria-live="polite" className="text-sm text-muted-foreground">
        {ANSWER_TEXT[lastAnswer]}
      </p>
    </div>
  );
}
