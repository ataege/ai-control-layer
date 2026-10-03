// Checks on the PostgreSQL address from the root .env, shared by test:db and reset:demo.
import { lookup } from "node:dns/promises";
import { isIP, connect } from "node:net";

const DATABASE_CONNECT_TIMEOUT_MS = 3_000;

/** The database host and port from an environment, with the starter's defaults. */
export function databaseAddress(environment) {
  const host = environment.POSTGRES_HOST || "localhost";
  const port = Number(environment.POSTGRES_PORT || 5432);
  return { host, port, label: `${host}:${port}` };
}

/** Resolves true when a TCP connection to the database address opens in time. */
export function databaseIsReachable({ host, port }) {
  return new Promise((resolveReachable) => {
    const socket = connect({ host, port, timeout: DATABASE_CONNECT_TIMEOUT_MS });
    const finish = (reachable) => {
      socket.destroy();
      resolveReachable(reachable);
    };
    socket.once("connect", () => finish(true));
    socket.once("timeout", () => finish(false));
    socket.once("error", () => finish(false));
  });
}

const isLoopbackAddress = (address) =>
  isIP(address) === 4 ? address.startsWith("127.") : address === "::1";

/**
 * Resolves true only when every address the host resolves to is a loopback address,
 * so a destructive command cannot reach a database on another machine.
 */
export async function hostIsLoopback(host) {
  try {
    const addresses = await lookup(host, { all: true });
    return addresses.length > 0 && addresses.every(({ address }) => isLoopbackAddress(address));
  } catch {
    return false;
  }
}
