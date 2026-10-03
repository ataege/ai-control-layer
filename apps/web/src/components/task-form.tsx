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
import { Textarea } from "@workspace/ui/components/textarea";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";

export function TaskForm() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Create Task (Sample Form)</CardTitle>
        <CardDescription>Mock configuration form for demonstration purposes.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="policy">Policy</Label>
          <Select defaultValue="invoice-reconciliation">
            <SelectTrigger id="policy">
              <SelectValue placeholder="Select a policy" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="invoice-reconciliation">Invoice Reconciliation</SelectItem>
              <SelectItem value="expense-approval">Expense Approval</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-2">
          <Label htmlFor="reason">Business Reason</Label>
          <Textarea id="reason" placeholder="Explain why this task is needed..." />
        </div>

        <div className="space-y-2">
          <Label htmlFor="amount">Allowance Amount (USD)</Label>
          <Input id="amount" type="number" placeholder="5000" />
        </div>
      </CardContent>
      <CardFooter>
        <Button className="w-full">Start Task</Button>
      </CardFooter>
    </Card>
  );
}
