"use client";

import { useState } from "react";
import { InboxIcon } from "lucide-react";

import { Button } from "@workspace/ui/components/button";
import { EmptyState } from "@workspace/ui/components/empty-state";
import { ErrorState } from "@workspace/ui/components/error-state";
import { LoadingState } from "@workspace/ui/components/loading-state";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@workspace/ui/components/tabs";

type DemoState = "empty" | "loading" | "error";

/** Switches between the three state components; the buttons inside them change the tab. */
export function StateComponentsDemo() {
  const [activeState, setActiveState] = useState<DemoState>("empty");

  return (
    <Tabs value={activeState} onValueChange={(nextValue) => setActiveState(nextValue as DemoState)}>
      <TabsList>
        <TabsTrigger value="empty">Empty</TabsTrigger>
        <TabsTrigger value="loading">Loading</TabsTrigger>
        <TabsTrigger value="error">Error</TabsTrigger>
      </TabsList>
      <TabsContent value="empty">
        <EmptyState
          icon={<InboxIcon />}
          title="No example items"
          description="Sample text that explains why the list is empty and what to do next."
          action={
            <Button size="sm" onClick={() => setActiveState("loading")}>
              Show loading state
            </Button>
          }
        />
      </TabsContent>
      <TabsContent value="loading">
        <LoadingState label="Loading example items" className="rounded-xl border" />
      </TabsContent>
      <TabsContent value="error">
        <ErrorState
          title="Example items could not be loaded"
          description="Sample text that describes the problem. This error is a demonstration only."
          action={
            <Button variant="outline" size="sm" onClick={() => setActiveState("empty")}>
              Back to empty state
            </Button>
          }
        />
      </TabsContent>
    </Tabs>
  );
}
