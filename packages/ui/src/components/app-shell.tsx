"use client";

import * as React from "react";
import { MenuIcon, XIcon } from "lucide-react";
import { Dialog as DialogPrimitive } from "radix-ui";

import { Button } from "@workspace/ui/components/button";
import { cn } from "@workspace/ui/lib/utils";

interface AppShellNavigationItem {
  href: string;
  label: string;
  icon?: React.ReactNode;
}

interface AppShellLinkProps {
  href: string;
  className?: string;
  children: React.ReactNode;
  onClick?: () => void;
  "aria-current"?: "page";
}

interface AppShellProps {
  /** Name or logo shown above the navigation and in the mobile top bar. */
  brand: React.ReactNode;
  navigationItems: readonly AppShellNavigationItem[];
  /** The href of the item to mark as the current page. */
  activeHref?: string;
  /** Router-aware link component. Defaults to a plain anchor. */
  linkComponent?: React.ComponentType<AppShellLinkProps>;
  navigationLabel?: string;
  /** Optional content pinned to the bottom of the sidebar. */
  sidebarFooter?: React.ReactNode;
  children: React.ReactNode;
}

function PlainAnchor({ children, ...anchorProps }: AppShellLinkProps) {
  return <a {...anchorProps}>{children}</a>;
}

interface NavigationListProps {
  navigationItems: readonly AppShellNavigationItem[];
  activeHref?: string;
  linkComponent: React.ComponentType<AppShellLinkProps>;
  navigationLabel: string;
  onNavigate?: () => void;
}

function NavigationList({
  navigationItems,
  activeHref,
  linkComponent: LinkComponent,
  navigationLabel,
  onNavigate,
}: NavigationListProps) {
  return (
    <nav aria-label={navigationLabel} className="flex flex-col gap-1">
      {navigationItems.map((navigationItem) => {
        const isActive = navigationItem.href === activeHref;
        return (
          <LinkComponent
            key={navigationItem.href}
            href={navigationItem.href}
            onClick={onNavigate}
            aria-current={isActive ? "page" : undefined}
            className={cn(
              "flex items-center gap-2 rounded-lg px-2.5 py-2 text-sm font-medium text-sidebar-foreground/70 transition-colors outline-none hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-3 focus-visible:ring-sidebar-ring/50 [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
              isActive && "bg-sidebar-accent text-sidebar-accent-foreground",
            )}
          >
            {navigationItem.icon}
            <span className="truncate">{navigationItem.label}</span>
          </LinkComponent>
        );
      })}
    </nav>
  );
}

function AppShell({
  brand,
  navigationItems,
  activeHref,
  linkComponent = PlainAnchor,
  navigationLabel = "Main navigation",
  sidebarFooter,
  children,
}: AppShellProps) {
  const [isMobileNavigationOpen, setIsMobileNavigationOpen] = React.useState(false);

  // Close the mobile menu when the viewport reaches the desktop (md) layout,
  // otherwise the hidden modal keeps the page scroll-locked and inert.
  React.useEffect(() => {
    const desktopLayoutQuery = window.matchMedia("(min-width: 768px)");
    const closeMenuOnDesktopLayout = (event: MediaQueryListEvent) => {
      if (event.matches) {
        setIsMobileNavigationOpen(false);
      }
    };
    desktopLayoutQuery.addEventListener("change", closeMenuOnDesktopLayout);
    return () => desktopLayoutQuery.removeEventListener("change", closeMenuOnDesktopLayout);
  }, []);

  return (
    <div data-slot="app-shell" className="flex min-h-svh flex-col md:flex-row">
      {/* Desktop sidebar */}
      <aside className="sticky top-0 hidden h-svh w-60 shrink-0 flex-col gap-4 border-r border-sidebar-border bg-sidebar p-4 text-sidebar-foreground md:flex">
        <div className="flex h-8 items-center px-2.5 text-sm font-semibold">{brand}</div>
        <NavigationList
          navigationItems={navigationItems}
          activeHref={activeHref}
          linkComponent={linkComponent}
          navigationLabel={navigationLabel}
        />
        {sidebarFooter ? <div className="mt-auto">{sidebarFooter}</div> : null}
      </aside>

      {/* Mobile top bar with slide-in navigation */}
      <header className="sticky top-0 z-40 flex h-14 items-center justify-between gap-2 border-b bg-background/95 px-4 backdrop-blur md:hidden">
        <div className="flex min-w-0 items-center text-sm font-semibold">{brand}</div>
        <DialogPrimitive.Root
          open={isMobileNavigationOpen}
          onOpenChange={setIsMobileNavigationOpen}
        >
          <DialogPrimitive.Trigger asChild>
            <Button variant="outline" size="icon" aria-label="Open navigation menu">
              <MenuIcon aria-hidden="true" />
            </Button>
          </DialogPrimitive.Trigger>
          <DialogPrimitive.Portal>
            <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/10 duration-200 supports-backdrop-filter:backdrop-blur-xs md:hidden data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0" />
            <DialogPrimitive.Content
              aria-describedby={undefined}
              className="fixed inset-y-0 left-0 z-50 flex w-72 max-w-[85vw] flex-col gap-4 border-r border-sidebar-border bg-sidebar p-4 text-sidebar-foreground shadow-lg duration-200 outline-none md:hidden data-open:animate-in data-open:slide-in-from-left data-closed:animate-out data-closed:slide-out-to-left"
            >
              <div className="flex h-8 items-center justify-between gap-2 pl-2.5">
                <DialogPrimitive.Title className="text-sm font-semibold">
                  {brand}
                </DialogPrimitive.Title>
                <DialogPrimitive.Close asChild>
                  <Button variant="ghost" size="icon-sm" aria-label="Close navigation menu">
                    <XIcon aria-hidden="true" />
                  </Button>
                </DialogPrimitive.Close>
              </div>
              <NavigationList
                navigationItems={navigationItems}
                activeHref={activeHref}
                linkComponent={linkComponent}
                navigationLabel={navigationLabel}
                onNavigate={() => setIsMobileNavigationOpen(false)}
              />
              {sidebarFooter ? <div className="mt-auto">{sidebarFooter}</div> : null}
            </DialogPrimitive.Content>
          </DialogPrimitive.Portal>
        </DialogPrimitive.Root>
      </header>

      <main className="min-w-0 flex-1">
        <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-4 py-6 md:px-8 md:py-8">
          {children}
        </div>
      </main>
    </div>
  );
}

export { AppShell };
export type { AppShellLinkProps, AppShellNavigationItem, AppShellProps };
