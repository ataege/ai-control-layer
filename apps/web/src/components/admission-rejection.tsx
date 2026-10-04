import { Button } from "@workspace/ui/components/button";
import {
  explainAdmissionRejection,
  REQUEST_FIELD_LABELS,
  type AdmissionRejection,
} from "@/lib/admission-rejection";

interface AdmissionRejectionNoticeProps {
  rejection: AdmissionRejection;
  /** Called only when the operator presses the button; nothing resubmits on its own. */
  onResubmit: () => void;
  /** True while a submission is in flight. */
  isSubmitting?: boolean;
}

// Shown in place of a passport when admission rejects a request: no passport or run exists. The form keeps
// the operator's choices, nothing is narrowed for them, and a revised request is a new, explicit submission.
// A code that admission does not return renders nothing, so this is never a generic error banner.
export function AdmissionRejectionNotice({
  rejection,
  onResubmit,
  isSubmitting = false,
}: AdmissionRejectionNoticeProps) {
  const explanation = explainAdmissionRejection(rejection.code);
  if (explanation === null) return null;

  return (
    <div
      role="alert"
      data-admission-rejection={rejection.code}
      className="space-y-3 rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm"
    >
      <p className="font-medium text-destructive">
        Request rejected at admission: no passport was issued and no run started.
      </p>
      <p>{explanation.unavailable}</p>
      <dl className="space-y-1">
        <div className="flex gap-2">
          <dt className="font-medium">Reason code</dt>
          <dd className="font-mono">{rejection.code}</dd>
        </div>
        <div className="flex gap-2">
          <dt className="font-medium">Admission said</dt>
          <dd>{explanation.safeMessage}</dd>
        </div>
        <div className="flex gap-2">
          <dt className="font-medium">Change</dt>
          <dd>
            {explanation.fieldsToChange.map((field) => REQUEST_FIELD_LABELS[field]).join(", ")}
          </dd>
        </div>
      </dl>
      <p>{explanation.whatToChange}</p>
      <p className="text-muted-foreground">
        Your choices are kept exactly as you made them. Nothing was narrowed for you; edit the
        fields above, then submit the revised request yourself.
      </p>
      <Button type="button" variant="outline" onClick={onResubmit} disabled={isSubmitting}>
        Submit revised request
      </Button>
    </div>
  );
}
