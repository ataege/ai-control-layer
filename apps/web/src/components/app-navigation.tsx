"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ActivityIcon, HouseIcon, LayoutGridIcon } from "lucide-react";

import { AppShell, type AppShellNavigationItem } from "@workspace/ui/components/app-shell";

import * as React from "react";
import { ProductClient } from "@/lib/product-client";
import { LogOutIcon } from "lucide-react";
import { useRouter } from "next/navigation";

const NAVIGATION_ITEMS: readonly AppShellNavigationItem[] = [
  { href: "/", label: "Home", icon: <HouseIcon aria-hidden="true" /> },
  { href: "/tasks/new", label: "New Task", icon: <LayoutGridIcon aria-hidden="true" /> },
  { href: "/components", label: "Components", icon: <LayoutGridIcon aria-hidden="true" /> },
  { href: "/diagnostics", label: "Diagnostics", icon: <ActivityIcon aria-hidden="true" /> },
  { href: "/judge", label: "Judge", icon: <LayoutGridIcon aria-hidden="true" /> },
  { href: "/security", label: "Security", icon: <ActivityIcon aria-hidden="true" /> },
];

/** Connects the generic AppShell to the Next.js router. */
export function AppNavigation({ children }: Readonly<{ children: React.ReactNode }>) {
  const currentPathname = usePathname();
  const router = useRouter();

  const [user, setUser] = React.useState<{
    name: string;
    email: string;
    organizationId: string;
  } | null>(null);

  React.useEffect(() => {
    if (currentPathname !== "/login") {
      ProductClient.getMe()
        .then((res) => {
          if (res.ok) {
            setUser(res.data);
          } else {
            setUser(null);
            // Redirect to login if unauthenticated on a protected route
            const isPublic =
              currentPathname.startsWith("/diagnostics") ||
              currentPathname.startsWith("/components");
            if (!isPublic) {
              router.push("/login");
            }
          }
        })
        .catch(() => setUser(null));
    }
  }, [currentPathname, router]);

  if (currentPathname === "/login") {
    return <main>{children}</main>;
  }

  const handleSignOut = async () => {
    await fetch("/api/auth/sign-out", { method: "POST" });
    router.push("/login");
    router.refresh();
  };

  const brand = (
    <div className="flex flex-col">
      <span className="font-bold">Task Passport</span>
      <span className="text-[10px] font-medium tracking-wider text-amber-500 uppercase">
        Development Demonstration
      </span>
    </div>
  );

  const sidebarFooter = user ? (
    <div className="flex items-center justify-between rounded-lg border border-sidebar-border bg-sidebar-accent/50 p-3">
      <div className="flex min-w-0 flex-col">
        <span className="truncate text-sm font-semibold">{user.name}</span>
        <span className="truncate text-xs text-muted-foreground">{user.email}</span>
        <span className="mt-1 truncate text-[10px] text-muted-foreground/80 uppercase">
          Org: {user.organizationId}
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
  ) : null;

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
