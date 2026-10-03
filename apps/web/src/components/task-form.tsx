"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { ProductClient, getSafeMessage, getErrorCode } from "@/lib/product-client";
import { Button } from "@workspace/ui/components/button";
import { Input } from "@workspace/ui/components/input";
import { Label } from "@workspace/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select";
import { Checkbox } from "@workspace/ui/components/checkbox";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { Loader2 } from "lucide-react";
import type { TaskFormOptions } from "@workspace/contracts";

export function TaskForm() {
  const router = useRouter();
  const [options, setOptions] = React.useState<TaskFormOptions | null>(null);
  const [isLoading, setIsLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [errorCode, setErrorCode] = React.useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = React.useState(false);

  // Form State
  const [template, setTemplate] = React.useState<string>("");
  const [vendorId, setVendorId] = React.useState<string>("");
  // Multiple invoices selected
  const [invoiceIds, setInvoiceIds] = React.useState<string[]>([]);
  const [destination, setDestination] = React.useState<string>("");
  const [approvalRequirement, setApprovalRequirement] = React.useState<string>("");
  const [modelCalls, setModelCalls] = React.useState<string>("");
  const [timeoutSeconds, setTimeoutSeconds] = React.useState<string>("");

  React.useEffect(() => {
    async function loadOptions() {
      setIsLoading(true);
      setError(null);
      try {
        const result = await ProductClient.getOptions();
        if (result.ok) {
          setOptions(result.data);
          if (result.data.templates.length > 0) setTemplate(result.data.templates[0]?.id || "");
          if (result.data.vendors.length > 0) setVendorId(result.data.vendors[0]?.id || "");
          if (result.data.destinations.length > 0)
            setDestination(result.data.destinations[0]?.id || "");
          if (result.data.approvalRequirements.length > 0)
            setApprovalRequirement(result.data.approvalRequirements[0]?.id || "");
          setModelCalls(result.data.limits.maxModelCalls.toString());
          setTimeoutSeconds(result.data.limits.maxTimeoutSeconds.toString());
        } else {
          setError(getSafeMessage(result.error));
        }
      } catch {
        setError("Failed to load task options.");
      } finally {
        setIsLoading(false);
      }
    }
    loadOptions();
  }, []);

  const toggleInvoice = (id: string) => {
    setInvoiceIds((prev) => (prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id]));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (invoiceIds.length === 0) {
      setError("Please select at least one invoice.");
      return;
    }

    setIsSubmitting(true);
    setError(null);
    setErrorCode(null);

    try {
      const result = await ProductClient.startRun({
        template,
        vendorId: vendorId || undefined,
        invoiceIds,
        destination,
        approvalRequirement: approvalRequirement || undefined,
        limits: {
          modelCalls: modelCalls ? parseInt(modelCalls, 10) : undefined,
          timeoutSeconds: timeoutSeconds ? parseInt(timeoutSeconds, 10) : undefined,
        },
      });

      if (!result.ok) {
        setError(getSafeMessage(result.error));
        setErrorCode(getErrorCode(result.error) || null);
        return;
      }

      router.push(`/runs/${encodeURIComponent(result.data.runId)}`);
    } catch {
      setError("An unexpected error occurred while starting the task.");
    } finally {
      setIsSubmitting(false);
    }
  };

  if (isLoading) {
    return (
      <Card>
        <CardContent className="flex items-center justify-center p-12">
          <Loader2 className="size-8 animate-spin text-muted-foreground" />
        </CardContent>
      </Card>
    );
  }

  if (error && !options) {
    return (
      <Card>
        <CardContent className="p-6 text-center text-destructive">
          <p>{error}</p>
          <Button variant="outline" className="mt-4" onClick={() => window.location.reload()}>
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (!options) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>Create Task</CardTitle>
        <CardDescription>Configure and delegate a new task.</CardDescription>
      </CardHeader>
      <form onSubmit={handleSubmit}>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label htmlFor="template">Template</Label>
            <Select value={template} onValueChange={setTemplate} required>
              <SelectTrigger id="template">
                <SelectValue placeholder="Select a template" />
              </SelectTrigger>
              <SelectContent>
                {options.templates.map((t) => (
                  <SelectItem key={t.id} value={t.id}>
                    {t.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="vendor">Vendor</Label>
            <Select value={vendorId} onValueChange={setVendorId}>
              <SelectTrigger id="vendor">
                <SelectValue placeholder="Select a vendor (optional)" />
              </SelectTrigger>
              <SelectContent>
                {options.vendors.map((v) => (
                  <SelectItem key={v.id} value={v.id}>
                    {v.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-3">
            <Label>Invoices</Label>
            <div className="max-h-48 space-y-2 overflow-y-auto rounded-md border p-3">
              {options.invoices.length === 0 && (
                <p className="text-sm text-muted-foreground">No invoices available.</p>
              )}
              {options.invoices.map((inv) => (
                <div key={inv.id} className="flex items-center space-x-2">
                  <Checkbox
                    id={`inv-${inv.id}`}
                    checked={invoiceIds.includes(inv.id)}
                    onCheckedChange={() => toggleInvoice(inv.id)}
                  />
                  <Label htmlFor={`inv-${inv.id}`} className="cursor-pointer font-normal">
                    {inv.number} - {inv.date} (${inv.amount})
                  </Label>
                </div>
              ))}
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="destination">Destination</Label>
            <Select value={destination} onValueChange={setDestination} required>
              <SelectTrigger id="destination">
                <SelectValue placeholder="Select a destination" />
              </SelectTrigger>
              <SelectContent>
                {options.destinations.map((d) => (
                  <SelectItem key={d.id} value={d.id}>
                    {d.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="approval">Approval Requirement</Label>
            <Select value={approvalRequirement} onValueChange={setApprovalRequirement}>
              <SelectTrigger id="approval">
                <SelectValue placeholder="Select requirement (optional)" />
              </SelectTrigger>
              <SelectContent>
                {options.approvalRequirements.map((a) => (
                  <SelectItem key={a.id} value={a.id}>
                    {a.description}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="modelCalls">Max Model Calls</Label>
              <Input
                id="modelCalls"
                type="number"
                min="1"
                max={options.limits.maxModelCalls}
                value={modelCalls}
                onChange={(e) => setModelCalls(e.target.value)}
                placeholder={options.limits.maxModelCalls.toString()}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="timeout">Timeout (Seconds)</Label>
              <Input
                id="timeout"
                type="number"
                min="1"
                max={options.limits.maxTimeoutSeconds}
                value={timeoutSeconds}
                onChange={(e) => setTimeoutSeconds(e.target.value)}
                placeholder={options.limits.maxTimeoutSeconds.toString()}
              />
            </div>
          </div>

          {error && (
            <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
              <div className="font-semibold mb-1">Admission Rejected</div>
              <div>{error}</div>
              {errorCode && (
                <div className="mt-2 text-xs bg-destructive/10 inline-block px-2 py-1 rounded font-mono">
                  Authority / Scope Limit: {errorCode}
                </div>
              )}
              <div className="mt-2 text-xs opacity-80">
                Please narrow your request scope or limits and resubmit.
              </div>
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
