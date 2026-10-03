import type { Metadata } from "next";

import "@workspace/ui/globals.css";
import { TooltipProvider } from "@workspace/ui/components/tooltip";

import { AppNavigation } from "@/components/app-navigation";

export const metadata: Metadata = {
  title: { default: "Task Passport", template: "%s | Task Passport" },
  description: "Control and monitor delegated agentic workflows.",
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
