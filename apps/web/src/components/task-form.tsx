"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import type { TaskFormOptions } from "@workspace/contracts";

import { AdmissionRejectionNotice } from "@/components/admission-rejection";
import { admissionRejectionFromError, type AdmissionRejection } from "@/lib/admission-rejection";
import { ProductClient } from "@/lib/product-client";
import {
  buildStartRunRequest,
  describeEmptyOffers,
  describeOptionsFailure,
  describeStartFailure,
  EMPTY_FORM_STATE,
  firstProblemWithChoices,
  formatInvoiceAmount,
  groupInvoicesByVendor,
  initialFormState,
  isInvoiceOutsideSelectedVendor,
  type OfferedInvoice,
  type TaskFormState,
} from "@/lib/task-form-model";
import { Button } from "@workspace/ui/components/button";
import { Checkbox } from "@workspace/ui/components/checkbox";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { Input } from "@workspace/ui/components/input";
import { Label } from "@workspace/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select";

// A Select item cannot have an empty value, so an optional choice left empty is this value.
const NOT_CHOSEN = "__not_chosen__";

interface TaskFormViewProps {
  options: TaskFormOptions;
  state: TaskFormState;
  /** Called with the one field the operator changed; no other field is touched. */
  onChange: (patch: Partial<TaskFormState>) => void;
  /** Called only from the operator's own submit or from the rejection notice's button. */
  onSubmit: () => void;
  isSubmitting: boolean;
  /** A reason the choices cannot be submitted, or a start failure that is not an admission rejection. */
  problem: string | null;
  /** The admission rejection of the last submission; no passport or run exists for it. */
  rejection: AdmissionRejection | null;
}

/** The form's fields and messages for a loaded options read; it holds no state of its own. */
export function TaskFormView({
  options,
  state,
  onChange,
  onSubmit,
  isSubmitting,
  problem,
  rejection,
}: TaskFormViewProps) {
  const offeredInvoices = options.invoices as OfferedInvoice[];
  const invoiceGroups = groupInvoicesByVendor(offeredInvoices, options.vendors);

  const toggleInvoice = (invoiceId: string) => {
    const invoiceIds = state.invoiceIds.includes(invoiceId)
      ? state.invoiceIds.filter((selectedId) => selectedId !== invoiceId)
      : [...state.invoiceIds, invoiceId];
    onChange({ invoiceIds });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Create Task</CardTitle>
        <CardDescription>
          Choose from what the server offers. Nothing is selected for you unless there is only one
          choice.
        </CardDescription>
      </CardHeader>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          onSubmit();
        }}
      >
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label htmlFor="template">Template</Label>
            <Select
              value={state.template}
              onValueChange={(template) => onChange({ template })}
              required
            >
              <SelectTrigger id="template">
                <SelectValue placeholder="Select a template" />
              </SelectTrigger>
              <SelectContent>
                {options.templates.map((template) => (
                  <SelectItem key={template.id} value={template.id}>
                    {template.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="vendor">Vendor (optional)</Label>
            <Select
              value={state.vendorId === "" ? NOT_CHOSEN : state.vendorId}
              onValueChange={(vendorId) =>
                onChange({ vendorId: vendorId === NOT_CHOSEN ? "" : vendorId })
              }
            >
              <SelectTrigger id="vendor">
                <SelectValue placeholder="Not specified" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NOT_CHOSEN}>Not specified</SelectItem>
                {options.vendors.map((vendor) => (
                  <SelectItem key={vendor.id} value={vendor.id}>
                    {vendor.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <fieldset className="space-y-3">
            <legend className="text-sm font-medium">Invoices</legend>
            <div className="max-h-64 space-y-4 overflow-y-auto rounded-md border p-3">
              {invoiceGroups.map((group) => (
                <div key={group.vendorId} className="space-y-2">
                  <p className="text-xs font-medium text-muted-foreground uppercase">
                    {group.vendorName}
                  </p>
                  {group.invoices.map((invoice) => {
                    const isOutsideSelectedVendor = isInvoiceOutsideSelectedVendor(
                      invoice,
                      offeredInvoices,
                      state.invoiceIds,
                    );
                    return (
                      <div key={invoice.id} className="flex items-center space-x-2">
                        <Checkbox
                          id={`invoice-${invoice.id}`}
                          checked={state.invoiceIds.includes(invoice.id)}
                          disabled={isOutsideSelectedVendor}
                          onCheckedChange={() => toggleInvoice(invoice.id)}
                        />
                        <Label
                          htmlFor={`invoice-${invoice.id}`}
                          className="cursor-pointer font-normal"
                        >
                          {invoice.number} · {invoice.date} ·{" "}
                          {formatInvoiceAmount(invoice.amount, invoice.currency)}
                        </Label>
                      </div>
                    );
                  })}
                </div>
              ))}
            </div>
            <p className="text-xs text-muted-foreground">
              A task covers the invoices of one vendor; invoices of another vendor are unavailable
              once one is selected.
            </p>
          </fieldset>

          <div className="space-y-2">
            <Label htmlFor="destination">Report destination</Label>
            <Select
              value={state.destination}
              onValueChange={(destination) => onChange({ destination })}
              required
            >
              <SelectTrigger id="destination">
                <SelectValue placeholder="Select a destination" />
              </SelectTrigger>
              <SelectContent>
                {options.destinations.map((destination) => (
                  <SelectItem key={destination.id} value={destination.id}>
                    {destination.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="approval">Approval requirement (optional)</Label>
            <Select
              value={state.approvalRequirement === "" ? NOT_CHOSEN : state.approvalRequirement}
              onValueChange={(approvalRequirement) =>
                onChange({
                  approvalRequirement:
                    approvalRequirement === NOT_CHOSEN ? "" : approvalRequirement,
                })
              }
            >
              <SelectTrigger id="approval">
                <SelectValue placeholder="Not specified" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NOT_CHOSEN}>Not specified</SelectItem>
                {options.approvalRequirements.map((requirement) => (
                  <SelectItem key={requirement.id} value={requirement.id}>
                    {requirement.description}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="modelCalls">Model calls limit (optional)</Label>
              <Input
                id="modelCalls"
                type="number"
                inputMode="numeric"
                min="1"
                value={state.modelCalls}
                onChange={(event) => onChange({ modelCalls: event.target.value })}
                placeholder={`up to ${options.limits.maxModelCalls}`}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="timeout">Timeout in seconds (optional)</Label>
              <Input
                id="timeout"
                type="number"
                inputMode="numeric"
                min="1"
                value={state.timeoutSeconds}
                onChange={(event) => onChange({ timeoutSeconds: event.target.value })}
                placeholder={`up to ${options.limits.maxTimeoutSeconds}`}
              />
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            Left empty, a limit is the server&apos;s own default. The figures are the highest the
            active policy accepts; a request above them is rejected at admission, with the reason.
          </p>

          {rejection !== null && (
            <AdmissionRejectionNotice
              rejection={rejection}
              onResubmit={onSubmit}
              isSubmitting={isSubmitting}
            />
          )}
          {problem !== null && (
            <div
              role="alert"
              className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive"
            >
              {problem}
            </div>
          )}
        </CardContent>
        <CardFooter>
          <Button type="submit" className="w-full" disabled={isSubmitting}>
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 size-4 animate-spin" /> Starting Task...
              </>
            ) : (
              "Start Task"
            )}
          </Button>
        </CardFooter>
      </form>
    </Card>
  );
}

type OptionsRead =
  | { status: "loading" }
  | { status: "failed"; message: string }
  | { status: "loaded"; options: TaskFormOptions };

export function TaskForm() {
  const router = useRouter();
  const [optionsRead, setOptionsRead] = React.useState<OptionsRead>({ status: "loading" });
  const [loadAttempt, setLoadAttempt] = React.useState(0);
  const [state, setState] = React.useState<TaskFormState>(EMPTY_FORM_STATE);
  const [isSubmitting, setIsSubmitting] = React.useState(false);
  const [problem, setProblem] = React.useState<string | null>(null);
  const [rejection, setRejection] = React.useState<AdmissionRejection | null>(null);

  React.useEffect(() => {
    let isCurrent = true;
    ProductClient.getOptions()
      .then((result) => {
        if (!isCurrent) return;
        if (result.ok) {
          setState(initialFormState(result.data));
          setOptionsRead({ status: "loaded", options: result.data });
        } else {
          setOptionsRead({ status: "failed", message: describeOptionsFailure(result.error) });
        }
      })
      .catch(() => {
        if (isCurrent) {
          setOptionsRead({ status: "failed", message: "Loading the task options failed." });
        }
      });
    return () => {
      isCurrent = false;
    };
  }, [loadAttempt]);

  const submit = async () => {
    if (optionsRead.status !== "loaded" || isSubmitting) return;
    const choicesProblem = firstProblemWithChoices(state, optionsRead.options);
    if (choicesProblem !== null) {
      setRejection(null);
      setProblem(choicesProblem);
      return;
    }

    setIsSubmitting(true);
    setProblem(null);
    setRejection(null);
    try {
      const result = await ProductClient.startRun(buildStartRunRequest(state));
      if (result.ok) {
        router.push(`/runs/${encodeURIComponent(result.data.runId)}`);
        return;
      }
      // The operator's choices stay exactly as they are; only a new submission can change the outcome.
      const admissionRejection = admissionRejectionFromError(result.error);
      if (admissionRejection !== null) setRejection(admissionRejection);
      else setProblem(describeStartFailure(result.error));
    } catch {
      setProblem("An unexpected error occurred, so it is not known whether the task was started.");
    } finally {
      setIsSubmitting(false);
    }
  };

  if (optionsRead.status === "loading") {
    return (
      <Card>
        <CardContent className="flex items-center justify-center p-12">
          <Loader2 className="size-8 animate-spin text-muted-foreground" aria-label="Loading" />
        </CardContent>
      </Card>
    );
  }

  const unavailableMessage =
    optionsRead.status === "failed"
      ? optionsRead.message
      : describeEmptyOffers(optionsRead.options);
  if (optionsRead.status === "failed" || unavailableMessage !== null) {
    return (
      <Card>
        <CardContent className="p-6 text-center">
          <p role="alert" className="text-destructive">
            {unavailableMessage}
          </p>
          <Button
            variant="outline"
            className="mt-4"
            onClick={() => {
              setOptionsRead({ status: "loading" });
              setLoadAttempt((attempt) => attempt + 1);
            }}
          >
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <TaskFormView
      options={optionsRead.options}
      state={state}
      onChange={(patch) => setState((current) => ({ ...current, ...patch }))}
      onSubmit={() => void submit()}
      isSubmitting={isSubmitting}
      problem={problem}
      rejection={rejection}
    />
  );
}
