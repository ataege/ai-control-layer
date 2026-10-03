# Product design: Task Passport

Two diagrams that describe the intended product. They are a design, not code: nothing shown here is
implemented in this repository. [docs/architecture.md](../architecture.md) describes what exists.

| File                                                               | Shows                                                                                                                                                                                                                               |
| ------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [task-passport-architecture.svg](task-passport-architecture.svg)   | Components and who owns what: the Next.js interface, the NestJS application service, the Go gateway as agent execution authority, one PostgreSQL with the schemas `app`, `runtime` and `demo`, and the LLM provider.                |
| [task-passport-run-lifecycle.svg](task-passport-run-lifecycle.svg) | The life of one run: task creation and admission, durable job claim with a lease, model allowance reservation and model call, policy decision, approval, recheck, tool execution with an idempotency key, retry or unknown outcome. |

Both files are rendered Mermaid exports. The Mermaid source is not in the repository; if you have
it, add it next to the SVGs as `.mmd` files so the design stays editable and diffable.

## Ownership the architecture diagram implies

- **NestJS** owns users, organization access, task templates, versioned policies, the tool catalog,
  the public API, the activity API with server-sent events and the runtime facade toward Go. It owns
  writes to the `app` schema and reads authorized events from the `runtime` schema.
- **Go** owns admission and the immutable task passport, the agent worker, the model gateway, the
  action gate, the approval manager, the tool executor, the typed tool adapters, data minimization
  and the runtime repository. It owns writes to the `runtime` schema, reads task and policy versions
  and revocations from `app`, and holds the LLM provider credentials. The tool adapters reach the
  `demo` schema with narrow database permissions.
- **Next.js** is the operator interface only.

## Open decisions between the starter and the design

Nobody has decided these yet. Write each outcome down here when it is settled.

1. **Schemas.** The starter uses the default `public` schema, including TypeORM's `migrations`
   table. The design uses `app`, `runtime` and `demo`. The first migration has to create them, and
   both services need to address them, either by qualified names or through a search path.
   Owner: nestjs with integration.
2. **Database roles.** The starter has one database user. The design needs writes limited per
   schema and narrow permissions for the tool adapters. Owner: infrastructure with nestjs and go.
3. **Browser to API path.** The Next.js proxy forwards no cookies or authorization headers and
   buffers a JSON response with a ten second timeout. Authenticated operator calls and the live
   activity stream need a different route design, or a direct connection from the browser to the
   API under the existing CORS setup. Owner: frontend with nestjs.
4. **Service identity.** The gateway ping proves reachability only. The design needs Go to verify
   the calling service and the operator context on internal calls. This is a contract change:
   integration decides the shape, go and nestjs implement it.
5. **Background worker in Go.** Claiming jobs with leases means a long-running worker inside the
   gateway. It has to be covered by graceful shutdown and by the readiness check. Owner: go.
6. **LLM provider credentials.** The design holds them in Go. That needs a new variable in
   `.env.example` (name only), read by Go alone and never by the web app or the API, and a team
   decision on which provider is used. Owner: infrastructure.
