import type { Metadata } from "next";
import Link from "next/link";
import { ActivityIcon, ArrowRightIcon, LayoutGridIcon } from "lucide-react";

import { Button } from "@workspace/ui/components/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { PageHeader } from "@workspace/ui/components/page-header";

export const metadata: Metadata = { title: "Home" };

// Parts of the starter listed under "What is included".
const STARTER_PARTS = [
  {
    name: "Web app",
    description: "Next.js App Router with a small server-side proxy to the API.",
  },
  {
    name: "API",
    description: "NestJS service with liveness, readiness and diagnostics endpoints.",
  },
  {
    name: "Gateway",
    description: "Go service that the API reaches with a service token.",
  },
  {
    name: "Database",
    description: "One PostgreSQL instance shared by the API and the gateway. No tables yet.",
  },
  {
    name: "Shared packages",
    description: "UI components, wire contracts, and TypeScript, ESLint and Prettier config.",
  },
] as const;

// Cards that link to the other pages of the app.
const PAGE_LINKS = [
  {
    href: "/components",
    title: "Component showcase",
    description: "Every shared primitive and generic component with neutral sample content.",
    icon: LayoutGridIcon,
  },
  {
    href: "/diagnostics",
    title: "Service diagnostics",
    description: "Live health of the API, the gateway and their database connections.",
    icon: ActivityIcon,
  },
] as const;

export default function HomePage() {
  return (
    <>
      <PageHeader
        title="Project starter"
        description="A generic full-stack foundation. It contains infrastructure and reusable building blocks only, ready for a product to be built on top."
      />

      {/* Links to the other pages */}
      <section aria-labelledby="pages-heading" className="flex flex-col gap-3">
        <h2 id="pages-heading" className="text-sm font-medium text-muted-foreground">
          Pages
        </h2>
        <div className="grid gap-4 sm:grid-cols-2">
          {PAGE_LINKS.map((pageLink) => (
            <Card key={pageLink.href}>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <pageLink.icon aria-hidden="true" className="size-4 text-muted-foreground" />
                  {pageLink.title}
                </CardTitle>
                <CardDescription>{pageLink.description}</CardDescription>
              </CardHeader>
              <CardFooter>
                <Button asChild variant="outline" size="sm">
                  <Link href={pageLink.href}>
                    Open
                    <ArrowRightIcon aria-hidden="true" data-icon="inline-end" />
                  </Link>
                </Button>
              </CardFooter>
            </Card>
          ))}
        </div>
      </section>

      {/* Static overview of the workspace; no live data here */}
      <section aria-labelledby="contents-heading" className="flex flex-col gap-3">
        <h2 id="contents-heading" className="text-sm font-medium text-muted-foreground">
          What is included
        </h2>
        <Card>
          <CardContent>
            <dl className="grid gap-x-6 gap-y-3 sm:grid-cols-[10rem_1fr]">
              {STARTER_PARTS.map((starterPart) => (
                <div key={starterPart.name} className="contents">
                  <dt className="font-medium">{starterPart.name}</dt>
                  <dd className="text-muted-foreground">{starterPart.description}</dd>
                </div>
              ))}
            </dl>
          </CardContent>
        </Card>
      </section>
    </>
  );
}
