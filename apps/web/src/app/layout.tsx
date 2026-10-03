import type { Metadata } from "next";

import "@workspace/ui/globals.css";
import { TooltipProvider } from "@workspace/ui/components/tooltip";

import { AppNavigation } from "@/components/app-navigation";

export const metadata: Metadata = {
  title: { default: "Starter", template: "%s | Starter" },
  description: "A generic full-stack project starter.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <TooltipProvider>
          <AppNavigation>{children}</AppNavigation>
        </TooltipProvider>
      </body>
    </html>
  );
}
