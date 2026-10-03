"use client";

import * as React from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ProductClient, getSafeMessage } from "@/lib/product-client";
import { Button } from "@workspace/ui/components/button";
import { Input } from "@workspace/ui/components/input";
import { Label } from "@workspace/ui/components/label";
import { ShieldAlert, Loader2, ArrowRight } from "lucide-react";

export function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [isLoading, setIsLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsLoading(true);

    try {
      const result = await ProductClient.signIn({ email, password });
      
      if (!result.ok) {
        setError(getSafeMessage(result.error));
        return;
      }

      // Check for a callback URL from middleware
      const callbackUrl = searchParams.get("callbackUrl");
      if (callbackUrl) {
        router.push(callbackUrl);
      } else {
        router.push("/");
      }
      
      router.refresh();
    } catch (err) {
      setError("An unexpected error occurred during sign in.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="relative overflow-hidden rounded-2xl border border-white/10 bg-background/50 p-8 shadow-2xl backdrop-blur-xl transition-all before:absolute before:inset-0 before:-z-10 before:bg-[radial-gradient(ellipse_at_top_right,_var(--tw-gradient-stops))] before:from-primary/10 before:via-background before:to-background">
      <div className="mb-8 flex flex-col items-center space-y-2 text-center">
        <div className="flex size-12 items-center justify-center rounded-full bg-primary/10 ring-1 ring-primary/20 mb-4">
          <ShieldAlert className="size-6 text-primary" aria-hidden="true" />
        </div>
        <h1 className="text-3xl font-bold tracking-tight text-foreground">Welcome back</h1>
        <p className="text-sm text-muted-foreground">
          Enter your credentials to access the control panel
        </p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="email">Email address</Label>
            <Input
              id="email"
              type="email"
              placeholder="operator@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={isLoading}
              required
              className="bg-background/50 backdrop-blur-sm transition-all focus:bg-background"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={isLoading}
              required
              className="bg-background/50 backdrop-blur-sm transition-all focus:bg-background"
            />
          </div>
        </div>

        {error && (
          <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive backdrop-blur-md">
            {error}
          </div>
        )}

        <Button
          type="submit"
          className="w-full group relative overflow-hidden transition-all hover:ring-2 hover:ring-primary/20 hover:ring-offset-2 hover:ring-offset-background"
          disabled={isLoading}
        >
          <span className="relative z-10 flex items-center gap-2">
            {isLoading ? (
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            ) : (
              <>
                Sign In
                <ArrowRight className="size-4 transition-transform group-hover:translate-x-1" aria-hidden="true" />
              </>
            )}
          </span>
        </Button>
      </form>
    </div>
  );
}
