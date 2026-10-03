import { Injectable } from "@nestjs/common";
import { type HealthIndicatorResult, HealthIndicatorService } from "@nestjs/terminus";
import { DataSource } from "typeorm";
import { AppConfigService } from "../config/app-config.service.js";

class DatabaseCheckTimeoutError extends Error {}

type ReleaseConnection = (discardReason?: unknown) => void;
type PooledConnection = [{ query(sql: string): Promise<unknown> }, ReleaseConnection];

/** The part of TypeORM's PostgresDriver the check needs: one pooled client and its release callback. */
interface PooledConnectionSource {
  obtainMasterConnection(): Promise<PooledConnection>;
}

/** Read-only `SELECT 1` bounded by DATABASE_TIMEOUT_MS. Failure messages are fixed, sanitized texts. */
@Injectable()
export class DatabaseHealthIndicator {
  constructor(
    private readonly dataSource: DataSource,
    private readonly healthIndicatorService: HealthIndicatorService,
    private readonly config: AppConfigService,
  ) {}

  async check(indicatorKey: string): Promise<HealthIndicatorResult> {
    const indicator = this.healthIndicatorService.check(indicatorKey);
    if (!this.dataSource.isInitialized) {
      return indicator.down({ message: "database connection not established" });
    }

    let timeoutTimer: NodeJS.Timeout | undefined;
    const timeoutExpired = new Promise<never>((_resolve, reject) => {
      timeoutTimer = setTimeout(
        () => reject(new DatabaseCheckTimeoutError()),
        this.config.databaseTimeoutMs,
      );
    });
    // Dedicated connection, so a failed check can discard it instead of leaving it checked out.
    const connectionSource = this.dataSource.driver as unknown as PooledConnectionSource;
    const connectionAttempt = connectionSource.obtainMasterConnection();
    let checkFailure: unknown;
    try {
      await Promise.race([
        connectionAttempt.then(([databaseClient]) => databaseClient.query("SELECT 1")),
        timeoutExpired,
      ]);
      return indicator.up();
    } catch (error) {
      checkFailure = error;
      // The raw driver error may name hosts or users, so it is never returned.
      const message =
        error instanceof DatabaseCheckTimeoutError
          ? "database check timed out"
          : "database unreachable";
      return indicator.down({ message });
    } finally {
      clearTimeout(timeoutTimer);
      // Releasing with the failure makes the pool destroy the connection rather than reuse it.
      // Chained on the attempt so a connection that arrives after the timeout is released too.
      void connectionAttempt.then(
        ([, releaseConnection]) => releaseConnection(checkFailure),
        () => undefined,
      );
    }
  }
}
