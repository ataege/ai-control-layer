#!/usr/bin/env node
// Smoke test against the RUNNING starter: node scripts/smoke.mjs [--mode=host|container]
// Uses real HTTP calls only. In container mode the gateway is not published, so its direct
// checks are reported as skipped. Secret values are never printed.
import { loadRootEnvironment, MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { probeHttp } from "./lib/http-probe.mjs";
import { countByStatus, printHeading, printResultTable } from "./lib/output.mjs";

const SMOKE_MODES = ["host", "container"];
const REQUEST_ID_HEADER = "x-request-id";
const PROXIED_API_PATHS = ["/api/health/live", "/api/health/ready", "/api/diagnostics/gateway"];
const WEB_PAGE_PATHS = ["/", "/components", "/diagnostics"];
// Shorter values could match unrelated page text, so they are not searched for.
const MINIMUM_SEARCHABLE_SECRET_LENGTH = 16;
// The diagnostics route makes two bounded upstream calls, so allow more than one timeout.
const PROBE_TIMEOUT_MS = 10_000;

// ------------------------------------------------------------ configuration

function parseMode(cliArguments) {
  let mode = "host";
  for (const cliArgument of cliArguments) {
    const modeValue = cliArgument.match(/^--mode=(.+)$/)?.[1];
    if (!modeValue || !SMOKE_MODES.includes(modeValue)) {
      console.error(`Usage: node scripts/smoke.mjs [--mode=${SMOKE_MODES.join("|")}]`);
      process.exit(2);
    }
    mode = modeValue;
  }
  return mode;
}

const mode = parseMode(process.argv.slice(2));
const { fileFound, environment } = loadRootEnvironment();
const serviceToken = environment.GATEWAY_SERVICE_TOKEN ?? "";
const databasePassword = environment.POSTGRES_PASSWORD ?? "";

if (!serviceToken || !databasePassword) {
  console.error(
    fileFound
      ? "[smoke] GATEWAY_SERVICE_TOKEN or POSTGRES_PASSWORD is empty. Run `pnpm run setup` to generate them."
      : `[smoke] ${MISSING_ENV_FILE_MESSAGE}`,
  );
  process.exit(1);
}

const webBaseUrl = `http://localhost:${environment.WEB_PORT || "3000"}`;
const apiBaseUrl = `http://localhost:${environment.API_PORT || "3001"}`;
const gatewayBaseUrl = `http://localhost:${environment.GATEWAY_PORT || "8080"}`;

// ------------------------------------------------------------------ results

const checkResults = [];
const pass = (name, detail) => checkResults.push({ name, status: "PASS", detail });
const fail = (name, detail) => checkResults.push({ name, status: "FAIL", detail });
const skip = (name, detail) => checkResults.push({ name, status: "SKIPPED", detail });

const probe = (url, headers = {}) => probeHttp(url, { headers, timeoutMs: PROBE_TIMEOUT_MS });

const describeProbe = (probeResult) =>
  probeResult.reached ? `HTTP ${probeResult.status}` : probeResult.failure;

/**
 * Records one check: the expected HTTP status plus an optional body predicate that
 * returns null when the body is right or a short description of what is wrong.
 */
function recordHttpCheck(name, probeResult, expectedStatus, describeBodyProblem) {
  if (!probeResult.reached) return fail(name, probeResult.failure);
  if (probeResult.status !== expectedStatus) {
    return fail(name, `expected HTTP ${expectedStatus}, got HTTP ${probeResult.status}`);
  }
  const bodyProblem = describeBodyProblem?.(probeResult.bodyJson) ?? null;
  if (bodyProblem) return fail(name, `HTTP ${probeResult.status}, but ${bodyProblem}`);
  return pass(name, `HTTP ${probeResult.status}`);
}

// ------------------------------------------------------------- body checks

const livenessProblem = (serviceName) => (body) =>
  body?.status === "ok" && body?.service === serviceName
    ? null
    : `body is not {status:"ok",service:"${serviceName}"}`;

function apiReadinessProblem(body) {
  if (body?.status !== "ok") return 'status is not "ok"';
  const indicators = Object.values(body.details ?? {});
  if (indicators.length === 0) return "no dependency is reported in details";
  if (body.details.database?.status !== "up") return "details.database is not up";
  return indicators.every((indicator) => indicator?.status === "up")
    ? null
    : "a dependency is not up";
}

function gatewayReadinessProblem(body) {
  if (body?.status !== "ok" || body?.service !== "gateway") return 'status is not "ok"';
  return body.checks?.database?.status === "up" ? null : "checks.database is not up";
}

function diagnosticsProblem(body) {
  if (body?.status !== "ok") return 'status is not "ok"';
  if (body.checks?.reachability?.status !== "up") return "checks.reachability is not up";
  if (body.checks?.databaseReadiness?.status !== "up") return "checks.databaseReadiness is not up";
  return null;
}

const errorCodeProblem = (expectedCode) => (body) =>
  body?.error?.code === expectedCode ? null : `error.code is not "${expectedCode}"`;

const pingProblem = (body) =>
  body?.status === "ok" && body?.service === "gateway" ? null : "unexpected ping body";

// ------------------------------------------------------------------ checks

/** Direct API checks. Returns the status per path for the proxy comparison. */
async function checkApi() {
  const [liveResult, readyResult, diagnosticsResult] = await Promise.all(
    PROXIED_API_PATHS.map((apiPath) => probe(`${apiBaseUrl}${apiPath}`)),
  );
  recordHttpCheck("api: GET /api/health/live", liveResult, 200, livenessProblem("api"));
  recordHttpCheck("api: GET /api/health/ready (database)", readyResult, 200, apiReadinessProblem);
  recordHttpCheck("api: GET /api/diagnostics/gateway", diagnosticsResult, 200, diagnosticsProblem);
  return [liveResult, readyResult, diagnosticsResult];
}

async function checkWebProxy(directApiResults) {
  for (const [pathIndex, apiPath] of PROXIED_API_PATHS.entries()) {
    const checkName = `web proxy: GET ${apiPath}`;
    const directResult = directApiResults[pathIndex];
    const proxiedResult = await probe(`${webBaseUrl}${apiPath}`);
    if (!proxiedResult.reached) fail(checkName, proxiedResult.failure);
    else if (!directResult.reached) {
      fail(checkName, `HTTP ${proxiedResult.status}, but the API is unreachable for comparison`);
    } else if (proxiedResult.status !== directResult.status) {
      fail(checkName, `HTTP ${proxiedResult.status}, but the API returned ${directResult.status}`);
    } else if (proxiedResult.bodyJson?.status !== directResult.bodyJson?.status) {
      fail(checkName, `HTTP ${proxiedResult.status}, but the body status differs from the API`);
    } else pass(checkName, `HTTP ${proxiedResult.status}, same as the API`);
  }
}

async function checkRequestIdEcho(targets) {
  for (const { label, url } of targets) {
    const checkName = `request id echoed: ${label}`;
    const requestId = `smoke-${Math.random().toString(36).slice(2, 12)}`;
    const probeResult = await probe(url, { [REQUEST_ID_HEADER]: requestId });
    if (!probeResult.reached) fail(checkName, probeResult.failure);
    else if (probeResult.headers.get(REQUEST_ID_HEADER) !== requestId) {
      fail(checkName, `HTTP ${probeResult.status}, but ${REQUEST_ID_HEADER} was not echoed`);
    } else pass(checkName, `HTTP ${probeResult.status}`);
  }
}

const GATEWAY_CHECK_NAMES = {
  live: "gateway: GET /health/live",
  ready: "gateway: GET /health/ready (database)",
  pingWithoutToken: "gateway: /internal/ping without token -> 401",
  pingWithWrongToken: "gateway: /internal/ping with wrong token -> 401",
  pingWithToken: "gateway: /internal/ping with service token -> 200",
};

async function checkGatewayDirectly() {
  const pingUrl = `${gatewayBaseUrl}/internal/ping`;
  const bearer = (token) => ({ authorization: `Bearer ${token}` });
  // Same length as the real token so only the value differs.
  const wrongToken = "x".repeat(serviceToken.length);

  recordHttpCheck(
    GATEWAY_CHECK_NAMES.live,
    await probe(`${gatewayBaseUrl}/health/live`),
    200,
    livenessProblem("gateway"),
  );
  recordHttpCheck(
    GATEWAY_CHECK_NAMES.ready,
    await probe(`${gatewayBaseUrl}/health/ready`),
    200,
    gatewayReadinessProblem,
  );
  recordHttpCheck(
    GATEWAY_CHECK_NAMES.pingWithoutToken,
    await probe(pingUrl),
    401,
    errorCodeProblem("unauthorized"),
  );
  recordHttpCheck(
    GATEWAY_CHECK_NAMES.pingWithWrongToken,
    await probe(pingUrl, bearer(wrongToken)),
    401,
    errorCodeProblem("unauthorized"),
  );
  recordHttpCheck(
    GATEWAY_CHECK_NAMES.pingWithToken,
    await probe(pingUrl, bearer(serviceToken)),
    200,
    pingProblem,
  );
}

/** Same-origin script and stylesheet URLs referenced anywhere in a page (tags or inlined data). */
function collectAssetUrls(pageHtml) {
  const assetUrls = new Set();
  const attributePattern = /(?:src|href)\s*=\s*["']([^"']+)["']/g;
  const nextStaticPattern = /\/_next\/static\/[A-Za-z0-9_\-./~%@[\]()]+?\.(?:js|css)/g;
  const candidates = [
    ...[...pageHtml.matchAll(attributePattern)].map((match) => match[1].replaceAll("&amp;", "&")),
    ...(pageHtml.match(nextStaticPattern) ?? []),
  ];
  for (const candidate of candidates) {
    let assetUrl;
    try {
      assetUrl = new URL(candidate, webBaseUrl);
    } catch {
      continue;
    }
    const isSameOrigin = assetUrl.origin === new URL(webBaseUrl).origin;
    if (isSameOrigin && /\.(?:js|css)$/.test(assetUrl.pathname)) assetUrls.add(assetUrl.href);
  }
  return assetUrls;
}

/** Fetches the pages and every asset they reference, then searches all of it for the secrets. */
async function checkWebPagesAndLeaks() {
  const fetchedDocuments = []; // { url, bodyText }
  const assetUrls = new Set();
  let everyPageLoaded = true;

  for (const pagePath of WEB_PAGE_PATHS) {
    const pageResult = await probe(`${webBaseUrl}${pagePath}`);
    recordHttpCheck(`web: GET ${pagePath}`, pageResult, 200);
    if (!pageResult.reached || pageResult.status !== 200) {
      everyPageLoaded = false;
      continue;
    }
    fetchedDocuments.push({ url: pagePath, bodyText: pageResult.bodyText });
    for (const assetUrl of collectAssetUrls(pageResult.bodyText)) assetUrls.add(assetUrl);
  }

  const failedAssetPaths = [];
  const assetResults = await Promise.all([...assetUrls].map((assetUrl) => probe(assetUrl)));
  [...assetUrls].forEach((assetUrl, assetIndex) => {
    const assetResult = assetResults[assetIndex];
    const assetPath = new URL(assetUrl).pathname;
    if (assetResult.reached && assetResult.status === 200) {
      fetchedDocuments.push({ url: assetPath, bodyText: assetResult.bodyText });
    } else failedAssetPaths.push(`${assetPath} (${describeProbe(assetResult)})`);
  });

  const secrets = [
    { label: "service token", value: serviceToken },
    { label: "database password", value: databasePassword },
  ];
  for (const { label, value } of secrets) {
    const checkName = `leak check: no ${label} in web pages and assets`;
    if (value.length < MINIMUM_SEARCHABLE_SECRET_LENGTH) {
      skip(checkName, `value is shorter than ${MINIMUM_SEARCHABLE_SECRET_LENGTH} characters`);
      continue;
    }
    const leakingUrls = fetchedDocuments
      .filter((fetchedDocument) => fetchedDocument.bodyText.includes(value))
      .map((fetchedDocument) => fetchedDocument.url);
    if (leakingUrls.length > 0) fail(checkName, `found in ${leakingUrls.join(", ")}`);
    else if (!everyPageLoaded) fail(checkName, "not every page could be loaded");
    else if (failedAssetPaths.length > 0) {
      fail(checkName, `could not load ${failedAssetPaths.slice(0, 3).join(", ")}`);
    } else if (assetUrls.size === 0) fail(checkName, "the pages reference no JS or CSS assets");
    else pass(checkName, `${WEB_PAGE_PATHS.length} pages and ${assetUrls.size} assets scanned`);
  }
}

// -------------------------------------------------------------------- main

console.log(`Smoke test (${mode} mode): web ${webBaseUrl}, api ${apiBaseUrl}`);

const directApiResults = await checkApi();
await checkWebPagesAndLeaks();
await checkWebProxy(directApiResults);

const requestIdTargets = [
  { label: "api", url: `${apiBaseUrl}/api/health/live` },
  { label: "web proxy", url: `${webBaseUrl}/api/health/live` },
];
if (mode === "host") {
  requestIdTargets.push({ label: "gateway", url: `${gatewayBaseUrl}/health/live` });
}
await checkRequestIdEcho(requestIdTargets);

if (mode === "host") {
  await checkGatewayDirectly();
} else {
  for (const checkName of [...Object.values(GATEWAY_CHECK_NAMES), "request id echoed: gateway"]) {
    skip(checkName, "gateway port is not published in container mode");
  }
}

printHeading("Smoke test summary");
printResultTable("CHECK", checkResults);
const counts = countByStatus(checkResults);
console.log(`\n${counts.PASS} passed, ${counts.FAIL} failed, ${counts.SKIPPED} skipped`);
process.exit(counts.FAIL === 0 ? 0 : 1);
