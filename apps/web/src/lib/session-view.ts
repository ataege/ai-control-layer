// What the navigation does with the signed-in operator (WEB-04): show them, label the seeded one as a
// development demonstration, and send a visitor whose session is gone to sign in. Pure, so each case
// is tested without a browser.

import type { FetchJsonResult } from "./fetch-json";
import { isPublicPath } from "./public-paths";

/** The operator `/api/auth/me` returns. The flag is the server's own mark of a development identity. */
export interface OperatorSession {
  name: string;
  email: string;
  organizationId: string;
  developmentDemonstration?: boolean;
}

export type SessionOutcome =
  | { status: "signed_in"; operator: OperatorSession }
  /** The server said there is no valid session (401): the visitor must sign in. */
  | { status: "signed_out" }
  /** The session could not be read (an unfinished route, a server or network failure): unknown, not signed out. */
  | { status: "unavailable" };

export function sessionOutcome(result: FetchJsonResult<OperatorSession>): SessionOutcome {
  if (result.ok) return { status: "signed_in", operator: result.data };
  const { error } = result;
  return error.kind === "http" && error.status === 401
    ? { status: "signed_out" }
    : { status: "unavailable" };
}

export { isPublicPath };

/**
 * An expired or missing session on a product page leads to sign-in, never to an empty page. A session
 * that merely could not be read does not: signing the operator out because a route failed would loop.
 */
export function mustSignIn(outcome: SessionOutcome, pathname: string): boolean {
  return outcome.status === "signed_out" && !isPublicPath(pathname);
}
