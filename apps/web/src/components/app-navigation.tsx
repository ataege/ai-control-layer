"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ActivityIcon, HouseIcon, LayoutGridIcon } from "lucide-react";

import { AppShell, type AppShellNavigationItem } from "@workspace/ui/components/app-shell";

const NAVIGATION_ITEMS: readonly AppShellNavigationItem[] = [
  { href: "/", label: "Home", icon: <HouseIcon aria-hidden="true" /> },
  { href: "/components", label: "Components", icon: <LayoutGridIcon aria-hidden="true" /> },
  { href: "/diagnostics", label: "Diagnostics", icon: <ActivityIcon aria-hidden="true" /> },
];

/** Connects the generic AppShell to the Next.js router. */
export function AppNavigation({ children }: Readonly<{ children: React.ReactNode }>) {
  const currentPathname = usePathname();

  return (
    <AppShell
      brand="Starter"
      navigationItems={NAVIGATION_ITEMS}
      activeHref={currentPathname}
      linkComponent={Link}
    >
      {children}
    </AppShell>
  );
}
