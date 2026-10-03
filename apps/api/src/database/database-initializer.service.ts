import {
  Injectable,
  Logger,
  type OnApplicationBootstrap,
  type OnApplicationShutdown,
} from "@nestjs/common";
import { DataSource } from "typeorm";

const INITIAL_RETRY_DELAY_MS = 1_000;
const MAX_RETRY_DELAY_MS = 15_000;

/** Reduces a connection error to its driver code so hosts and credentials never reach the logs. */
export function describeConnectionFailure(error: unknown): string {
  const errorCode = (error as { code?: unknown } | null)?.code;
  return typeof errorCode === "string" && /^[A-Za-z0-9_]{1,32}$/.test(errorCode)
    ? errorCode
    : "connection_failed";
}

/**
 * Connects to PostgreSQL in the background so the HTTP server starts even when the database is down.
 * Retries with capped exponential backoff until connected or shut down. Never runs migrations.
 */
@Injectable()
export class DatabaseInitializerService implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(DatabaseInitializerService.name);
  private retryTimer?: NodeJS.Timeout;
  private nextRetryDelayMs = INITIAL_RETRY_DELAY_MS;
  private isShuttingDown = false;
  private pendingAttempt: Promise<void> = Promise.resolve();

  constructor(private readonly dataSource: DataSource) {}

  onApplicationBootstrap(): void {
    this.pendingAttempt = this.tryInitialize();
  }

  async onApplicationShutdown(): Promise<void> {
    this.isShuttingDown = true;
    clearTimeout(this.retryTimer);
    // Let an in-flight attempt settle so the connection pool can be closed.
    await this.pendingAttempt;
    if (this.dataSource.isInitialized) {
      await this.dataSource.destroy();
      this.logger.log("Database connection closed");
    }
  }

  private async tryInitialize(): Promise<void> {
    if (this.isShuttingDown || this.dataSource.isInitialized) {
      return;
    }
    try {
      await this.dataSource.initialize();
      this.nextRetryDelayMs = INITIAL_RETRY_DELAY_MS;
      this.logger.log("Database connection established");
    } catch (error) {
      if (this.isShuttingDown) {
        return;
      }
      const retryDelayMs = this.nextRetryDelayMs;
      this.nextRetryDelayMs = Math.min(retryDelayMs * 2, MAX_RETRY_DELAY_MS);
      this.logger.warn(
        `Database unavailable (${describeConnectionFailure(error)}), retrying in ${retryDelayMs}ms`,
      );
      this.retryTimer = setTimeout(() => {
        this.pendingAttempt = this.tryInitialize();
      }, retryDelayMs);
    }
  }
}
