import { describe, expect, it } from "vitest";

import nextConfig from "../../next.config";

async function headersOfEveryRoute(): Promise<Map<string, string>> {
  const rules = (await nextConfig.headers?.()) ?? [];
  const everyRoute = rules.filter((rule) => rule.source === "/:path*");
  return new Map(everyRoute.flatMap((rule) => rule.headers.map((h) => [h.key, h.value] as const)));
}

describe("security headers", () => {
  it("are set for every route", async () => {
    const headers = await headersOfEveryRoute();
    expect(headers.get("X-Content-Type-Options")).toBe("nosniff");
    expect(headers.get("Referrer-Policy")).toBe("no-referrer");
    expect(headers.get("X-Frame-Options")).toBe("DENY");
  });

  it("carry a policy that allows only this origin and no framing", async () => {
    const policy = (await headersOfEveryRoute()).get("Content-Security-Policy") ?? "";
    const directives = new Map(
      policy.split(";").map((part) => {
        const [name, ...values] = part.trim().split(/\s+/);
        return [name, values] as const;
      }),
    );
    expect(directives.get("default-src")).toEqual(["'self'"]);
    expect(directives.get("frame-ancestors")).toEqual(["'none'"]);
    expect(directives.get("object-src")).toEqual(["'none'"]);
    expect(directives.get("base-uri")).toEqual(["'self'"]);
    expect(directives.get("form-action")).toEqual(["'self'"]);
    // No directive names another host.
    expect(policy).not.toMatch(/https?:\/\/|\*/);
  });
});
