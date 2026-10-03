import type { Metadata } from "next";
import { CircleAlertIcon, InfoIcon, PlusIcon } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Badge } from "@workspace/ui/components/badge";
import { Button } from "@workspace/ui/components/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { Checkbox } from "@workspace/ui/components/checkbox";
import { CopyButton } from "@workspace/ui/components/copy-button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@workspace/ui/components/dialog";
import { Input } from "@workspace/ui/components/input";
import { Label } from "@workspace/ui/components/label";
import { PageHeader } from "@workspace/ui/components/page-header";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select";
import { Skeleton } from "@workspace/ui/components/skeleton";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@workspace/ui/components/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@workspace/ui/components/tabs";
import { Textarea } from "@workspace/ui/components/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@workspace/ui/components/tooltip";

import { ConfirmDialogDemo } from "./confirm-dialog-demo";
import { ShowcaseSection } from "./showcase-section";
import { StateComponentsDemo } from "./state-components-demo";

export const metadata: Metadata = { title: "Components" };

// Neutral rows for the Table section.
const SAMPLE_ROWS = [
  { name: "Example item A", category: "Sample", note: "First row of sample text" },
  { name: "Example item B", category: "Sample", note: "Second row of sample text" },
  { name: "Example item C", category: "Sample", note: "Third row of sample text" },
] as const;

// Value shown next to the CopyButton examples.
const SAMPLE_COPY_VALUE = "sample-value-123";

// One ShowcaseSection per component, each with neutral sample content.
export default function ComponentsPage() {
  return (
    <>
      <PageHeader
        title="Component showcase"
        description="Shared primitives and generic components from the UI package, shown with neutral sample content."
      />

      {/* shadcn/ui primitives */}
      <ShowcaseSection title="Button" description="Variants, sizes, icons and the disabled state.">
        <div className="flex flex-wrap items-center gap-2">
          <Button>Default</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="outline">Outline</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="destructive">Destructive</Button>
          <Button variant="link">Link</Button>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button size="xs">Extra small</Button>
          <Button size="sm">Small</Button>
          <Button size="lg">Large</Button>
          <Button>
            <PlusIcon aria-hidden="true" data-icon="inline-start" />
            With icon
          </Button>
          <Button size="icon" variant="outline" aria-label="Add example item">
            <PlusIcon aria-hidden="true" />
          </Button>
          <Button disabled>Disabled</Button>
        </div>
      </ShowcaseSection>

      <ShowcaseSection title="Badge" description="Short status or category labels.">
        <div className="flex flex-wrap items-center gap-2">
          <Badge>Default</Badge>
          <Badge variant="secondary">Secondary</Badge>
          <Badge variant="outline">Outline</Badge>
          <Badge variant="destructive">Destructive</Badge>
        </div>
      </ShowcaseSection>

      <ShowcaseSection
        title="Form controls"
        description="Input, Textarea, Label, Select and Checkbox."
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-2">
            <Label htmlFor="sample-text-input">Text input</Label>
            <Input id="sample-text-input" placeholder="Sample text" />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="sample-invalid-input">Invalid input</Label>
            <Input id="sample-invalid-input" defaultValue="Sample text" aria-invalid="true" />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="sample-select">Select</Label>
            <Select>
              <SelectTrigger id="sample-select" className="w-full">
                <SelectValue placeholder="Choose an option" />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectLabel>Sample options</SelectLabel>
                  <SelectItem value="option-one">Option one</SelectItem>
                  <SelectItem value="option-two">Option two</SelectItem>
                  <SelectItem value="option-three">Option three</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="sample-disabled-input">Disabled input</Label>
            <Input id="sample-disabled-input" placeholder="Sample text" disabled />
          </div>
          <div className="flex flex-col gap-2 sm:col-span-2">
            <Label htmlFor="sample-textarea">Textarea</Label>
            <Textarea id="sample-textarea" placeholder="Longer sample text" />
          </div>
          <div className="flex items-center gap-2">
            <Checkbox id="sample-checkbox" defaultChecked />
            <Label htmlFor="sample-checkbox">Checkbox with a label</Label>
          </div>
        </div>
      </ShowcaseSection>

      <ShowcaseSection title="Card" description="Header, action, content and footer slots.">
        <Card className="max-w-sm">
          <CardHeader>
            <CardTitle>Example item</CardTitle>
            <CardDescription>Sample text for the card description.</CardDescription>
            <CardAction>
              <Badge variant="secondary">Sample</Badge>
            </CardAction>
          </CardHeader>
          <CardContent>Sample text for the card body.</CardContent>
          <CardFooter>
            <Button size="sm" variant="outline">
              Sample action
            </Button>
          </CardFooter>
        </Card>
      </ShowcaseSection>

      <ShowcaseSection title="Alert" description="Inline messages in two variants.">
        <Alert>
          <InfoIcon aria-hidden="true" />
          <AlertTitle>Example notice</AlertTitle>
          <AlertDescription>Sample text for an informational message.</AlertDescription>
        </Alert>
        <Alert variant="destructive">
          <CircleAlertIcon aria-hidden="true" />
          <AlertTitle>Example problem</AlertTitle>
          <AlertDescription>
            Sample text for a message about something that failed.
          </AlertDescription>
        </Alert>
      </ShowcaseSection>

      <ShowcaseSection title="Tabs" description="Switch between panels of content.">
        <Tabs defaultValue="first">
          <TabsList>
            <TabsTrigger value="first">First tab</TabsTrigger>
            <TabsTrigger value="second">Second tab</TabsTrigger>
          </TabsList>
          <TabsContent value="first">Sample text for the first panel.</TabsContent>
          <TabsContent value="second">Sample text for the second panel.</TabsContent>
        </Tabs>
      </ShowcaseSection>

      <ShowcaseSection title="Table" description="Rows of sample content.">
        <Table>
          <TableCaption>Three example items.</TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Category</TableHead>
              <TableHead>Note</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {SAMPLE_ROWS.map((sampleRow) => (
              <TableRow key={sampleRow.name}>
                <TableCell className="font-medium">{sampleRow.name}</TableCell>
                <TableCell>{sampleRow.category}</TableCell>
                <TableCell className="text-muted-foreground">{sampleRow.note}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </ShowcaseSection>

      <ShowcaseSection
        title="Skeleton"
        description="Placeholder shapes for content that is loading."
      >
        <div className="flex items-center gap-3">
          <Skeleton className="size-10 rounded-full" />
          <div className="flex flex-1 flex-col gap-2">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-4 w-1/3" />
          </div>
        </div>
      </ShowcaseSection>

      <ShowcaseSection title="Tooltip" description="Extra context on hover or keyboard focus.">
        <div>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline">Hover or focus</Button>
            </TooltipTrigger>
            <TooltipContent>Sample tooltip text</TooltipContent>
          </Tooltip>
        </div>
      </ShowcaseSection>

      <ShowcaseSection title="Dialog" description="A modal window with a title and a footer.">
        <div>
          <Dialog>
            <DialogTrigger asChild>
              <Button variant="outline">Open dialog</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Example dialog</DialogTitle>
                <DialogDescription>
                  Sample text that describes the dialog content.
                </DialogDescription>
              </DialogHeader>
              <DialogFooter showCloseButton />
            </DialogContent>
          </Dialog>
        </div>
      </ShowcaseSection>

      {/* Generic components built on the primitives; interactive demos are client components */}
      <ShowcaseSection
        title="ConfirmDialog"
        description="Asks for confirmation before an action. Built on AlertDialog."
      >
        <ConfirmDialogDemo />
      </ShowcaseSection>

      <ShowcaseSection title="CopyButton" description="Copies a value to the clipboard.">
        <div className="flex flex-wrap items-center gap-3">
          <code className="rounded-md bg-muted px-2 py-1 font-mono text-xs">
            {SAMPLE_COPY_VALUE}
          </code>
          <CopyButton value={SAMPLE_COPY_VALUE} />
          <CopyButton
            value={SAMPLE_COPY_VALUE}
            label="Copy sample value"
            iconOnly
            variant="ghost"
          />
        </div>
      </ShowcaseSection>

      <ShowcaseSection
        title="EmptyState, LoadingState and ErrorState"
        description="Placeholders for a region without content, with pending content, or with a failure."
      >
        <StateComponentsDemo />
      </ShowcaseSection>

      <ShowcaseSection title="PageHeader and AppShell" description="Page-level layout components.">
        <div className="rounded-xl border p-4">
          {/* Nested demo: h3 keeps the page at a single h1 */}
          <PageHeader
            headingLevel={3}
            title="Example page"
            description="Sample text for the page description."
            actions={<Button size="sm">Sample action</Button>}
          />
        </div>
        <p className="text-sm text-muted-foreground">
          AppShell is the frame around this page: a sidebar on wide screens, and a top bar with a
          menu button and slide-in navigation on narrow ones.
        </p>
      </ShowcaseSection>
    </>
  );
}
