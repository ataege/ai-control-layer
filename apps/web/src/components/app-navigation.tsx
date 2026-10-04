"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ActivityIcon, HouseIcon, LayoutGridIcon } from "lucide-react";

import { AppShell, type AppShellNavigationItem } from "@workspace/ui/components/app-shell";

import * as React from "react";
import { LogOutIcon } from "lucide-react";
import { useRouter } from "next/navigation";

import { DevelopmentDemonstrationLabel } from "@/components/labels";
import { ProductClient } from "@/lib/product-client";
import { mustSignIn, sessionOutcome, type SessionOutcome } from "@/lib/session-view";

const NAVIGATION_ITEMS: readonly AppShellNavigationItem[] = [
  { href: "/", label: "Home", icon: <HouseIcon aria-hidden="true" /> },
  { href: "/tasks/new", label: "New Task", icon: <LayoutGridIcon aria-hidden="true" /> },
  { href: "/components", label: "Components", icon: <LayoutGridIcon aria-hidden="true" /> },
  { href: "/diagnostics", label: "Diagnostics", icon: <ActivityIcon aria-hidden="true" /> },
  { href: "/judge", label: "Judge", icon: <LayoutGridIcon aria-hidden="true" /> },
  { href: "/security", label: "Security posture", icon: <ActivityIcon aria-hidden="true" /> },
];

/** Connects the generic AppShell to the Next.js router. */
export function AppNavigation({ children }: Readonly<{ children: React.ReactNode }>) {
  const currentPathname = usePathname();
  const router = useRouter();

  // null until the session has been read; "unavailable" when it could not be read at all.
  const [session, setSession] = React.useState<SessionOutcome | null>(null);

  React.useEffect(() => {
    if (currentPathname === "/login") return;
    let isCurrent = true;
    ProductClient.getMe()
      .then((result) => {
        if (!isCurrent) return;
        const outcome = sessionOutcome(result);
        setSession(outcome);
        // An expired or missing session on a product page leads to sign-in, never to an empty page.
        if (mustSignIn(outcome, currentPathname)) router.push("/login");
      })
      .catch(() => {
        if (isCurrent) setSession({ status: "unavailable" });
      });
    return () => {
      isCurrent = false;
    };
  }, [currentPathname, router]);

  if (currentPathname === "/login") {
    return <main>{children}</main>;
  }

  const handleSignOut = async () => {
    await fetch("/api/auth/sign-out", { method: "POST" });
    router.push("/login");
    router.refresh();
  };

  const operator = session?.status === "signed_in" ? session.operator : null;

  // The label comes from the session (the server's flag, else the seeded operator's email), so a
  // session that is not the development identity, or an unreadable one, never shows it.
  const brand = (
    <div className="flex flex-col gap-1">
      <span className="font-bold">Task Passport</span>
      <DevelopmentDemonstrationLabel
        email={operator?.email}
        developmentDemonstration={operator?.developmentDemonstration}
      />
    </div>
  );

  let sidebarFooter: React.ReactNode = null;
  if (operator !== null) {
    sidebarFooter = (
      <div className="flex items-center justify-between rounded-lg border border-sidebar-border bg-sidebar-accent/50 p-3">
        <div className="flex min-w-0 flex-col">
          <span className="truncate text-sm font-semibold">{operator.name}</span>
          <span className="truncate text-xs text-muted-foreground">{operator.email}</span>
          <span className="mt-1 truncate text-[10px] text-muted-foreground/80 uppercase">
            Organization: {operator.organizationId}
          </span>
        </div>
        <button
          onClick={handleSignOut}
          className="ml-2 shrink-0 rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-foreground"
          aria-label="Sign out"
        >
          <LogOutIcon className="size-4" />
        </button>
      </div>
    );
  } else if (session?.status === "unavailable") {
    sidebarFooter = (
      <p className="rounded-lg border border-sidebar-border p-3 text-xs text-muted-foreground">
        The signed-in operator could not be read.
      </p>
    );
  }

  return (
    <AppShell
      brand={brand}
      navigationItems={NAVIGATION_ITEMS}
      activeHref={currentPathname}
      linkComponent={Link}
      sidebarFooter={sidebarFooter}
    >
      {children}
    </AppShell>
  );
}
