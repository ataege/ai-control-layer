import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Paths that do not require authentication
  const isPublicPath =
    pathname.startsWith("/login") ||
    pathname.startsWith("/api") ||
    pathname.startsWith("/_next") ||
    pathname.startsWith("/health") ||
    pathname.startsWith("/diagnostics") ||
    pathname === "/favicon.ico";

  if (isPublicPath) {
    return NextResponse.next();
  }

  // Get the session cookie
  const session = request.cookies.get("session")?.value;

  // If no session is found, redirect to the login page
  if (!session) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("callbackUrl", pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    // Match all request paths except for the ones starting with:
    // - _next/static (static files)
    // - _next/image (image optimization files)
    // - favicon.ico (favicon file)
    "/((?!_next/static|_next/image|favicon.ico).*)",
  ],
};
