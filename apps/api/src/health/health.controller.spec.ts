import type { NestExpressApplication } from "@nestjs/platform-express";
import { TerminusModule } from "@nestjs/terminus";
import request from "supertest";
import { DataSource } from "typeorm";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppConfigService } from "../config/app-config.service.js";
import { createTestApp } from "../testing/create-test-app.js";
import { DatabaseHealthIndicator } from "./database.health-indicator.js";
import { HealthController } from "./health.controller.js";

const DATABASE_TIMEOUT_MS = 100;
// Default size of the pg connection pool; the leak regression test must exceed it.
const DEFAULT_POOL_SIZE = 10;

type RunQuery = () => Promise<unknown>;

interface DataSourceStub {
  isInitialized: boolean;
  driver: { obtainMasterConnection: () => Promise<[{ query: RunQuery }, () => void]> };
}

/** DataSource stub whose pooled connections run the given query; `releaseConnection` records releases. */
function createDataSourceStub(isInitialized: boolean, runQuery: RunQuery) {
  const releaseConnection = vi.fn<(discardReason?: unknown) => void>();
  const dataSourceStub: DataSourceStub = {
    isInitialized,
    driver: {
      obtainMasterConnection: () => Promise.resolve([{ query: runQuery }, releaseConnection]),
    },
  };
  return { dataSourceStub, releaseConnection };
}

describe("HealthController", () => {
  let app: NestExpressApplication | undefined;

  async function startAppWith(
    isInitialized: boolean,
    runQuery: RunQuery,
  ): Promise<{
    runningApp: NestExpressApplication;
    releaseConnection: ReturnType<typeof createDataSourceStub>["releaseConnection"];
  }> {
    const { dataSourceStub, releaseConnection } = createDataSourceStub(isInitialized, runQuery);
    return { runningApp: await startAppWithDataSource(dataSourceStub), releaseConnection };
  }

  async function startAppWithDataSource(
    dataSourceStub: DataSourceStub,
  ): Promise<NestExpressApplication> {
    app = await createTestApp({
      // The terminus logger is silenced: failed checks are expected here.
      imports: [TerminusModule.forRoot({ logger: false })],
      controllers: [HealthController],
      providers: [
        DatabaseHealthIndicator,
        { provide: DataSource, useValue: dataSourceStub },
        { provide: AppConfigService, useValue: { databaseTimeoutMs: DATABASE_TIMEOUT_MS } },
      ],
    });
    return app;
  }

  afterEach(async () => {
    await app?.close();
    app = undefined;
  });

  it("reports ready with 200 when the query succeeds", async () => {
    const { runningApp, releaseConnection } = await startAppWith(true, () =>
      Promise.resolve([{ "?column?": 1 }]),
    );

    const response = await request(runningApp.getHttpServer()).get("/api/health/ready");

    expect(response.status).toBe(200);
    expect(response.body).toEqual({
      status: "ok",
      info: { database: { status: "up" } },
      error: {},
      details: { database: { status: "up" } },
    });
    // A healthy connection goes back to the pool for reuse.
    expect(releaseConnection).toHaveBeenCalledExactlyOnceWith(undefined);
  });

  it("returns 503 with a sanitized report while the DataSource is not initialised", async () => {
    const { runningApp } = await startAppWith(false, () =>
      Promise.reject(new Error("must not be called")),
    );

    const response = await request(runningApp.getHttpServer()).get("/api/health/ready");

    expect(response.status).toBe(503);
    expect(response.body).toEqual({
      status: "error",
      info: {},
      error: { database: { status: "down", message: "database connection not established" } },
      details: { database: { status: "down", message: "database connection not established" } },
    });
  });

  it("returns 503 without driver details when the query throws", async () => {
    const { runningApp, releaseConnection } = await startAppWith(true, () =>
      Promise.reject(new Error("connect ECONNREFUSED 10.1.2.3:5432 user=starter password=hunter2")),
    );

    const response = await request(runningApp.getHttpServer()).get("/api/health/ready");

    expect(response.status).toBe(503);
    expect(response.body).toMatchObject({
      status: "error",
      error: { database: { status: "down", message: "database unreachable" } },
    });
    const serializedBody = JSON.stringify(response.body);
    expect(serializedBody).not.toContain("10.1.2.3");
    expect(serializedBody).not.toContain("hunter2");
    // Released with the failure, so the pool discards the broken connection.
    expect(releaseConnection).toHaveBeenCalledExactlyOnceWith(expect.any(Error));
  });

  it("returns 503 when the query exceeds DATABASE_TIMEOUT_MS", async () => {
    const { runningApp } = await startAppWith(
      true,
      () => new Promise((resolve) => setTimeout(resolve, DATABASE_TIMEOUT_MS * 10)),
    );

    const response = await request(runningApp.getHttpServer()).get("/api/health/ready");

    expect(response.status).toBe(503);
    expect(response.body).toMatchObject({
      error: { database: { status: "down", message: "database check timed out" } },
    });
  });

  it("discards the connection of every timed-out check and recovers once queries answer again", async () => {
    // Simulates a silently dropped connection: the query never settles.
    let databaseAnswers = false;
    const { runningApp, releaseConnection } = await startAppWith(true, () =>
      databaseAnswers ? Promise.resolve([{ "?column?": 1 }]) : new Promise(() => undefined),
    );
    const hungCheckCount = DEFAULT_POOL_SIZE + 2;

    for (let checkNumber = 1; checkNumber <= hungCheckCount; checkNumber += 1) {
      const hungResponse = await request(runningApp.getHttpServer()).get("/api/health/ready");

      expect(hungResponse.status).toBe(503);
      expect(hungResponse.body).toMatchObject({
        error: { database: { status: "down", message: "database check timed out" } },
      });
      // Without a release carrying the failure, each hung check would keep one pool slot forever.
      expect(releaseConnection).toHaveBeenCalledTimes(checkNumber);
      expect(releaseConnection).toHaveBeenLastCalledWith(expect.any(Error));
    }

    databaseAnswers = true;
    const recoveredResponse = await request(runningApp.getHttpServer()).get("/api/health/ready");

    expect(recoveredResponse.status).toBe(200);
    expect(releaseConnection).toHaveBeenLastCalledWith(undefined);
  });

  it("releases a connection that only arrives after the check timed out", async () => {
    // Simulates waiting for a pool slot longer than DATABASE_TIMEOUT_MS.
    const releaseLateConnection = vi.fn<(discardReason?: unknown) => void>();
    let deliverConnection: () => void = () => undefined;
    const lateDataSourceStub: DataSourceStub = {
      isInitialized: true,
      driver: {
        obtainMasterConnection: () =>
          new Promise((resolve) => {
            deliverConnection = () =>
              resolve([{ query: () => Promise.resolve([]) }, releaseLateConnection]);
          }),
      },
    };
    const runningApp = await startAppWithDataSource(lateDataSourceStub);

    const response = await request(runningApp.getHttpServer()).get("/api/health/ready");
    expect(response.status).toBe(503);
    expect(releaseLateConnection).not.toHaveBeenCalled();

    deliverConnection();
    await vi.waitFor(() => {
      expect(releaseLateConnection).toHaveBeenCalledExactlyOnceWith(expect.any(Error));
    });
  });

  it("keeps liveness at 200 while the database is down", async () => {
    const { runningApp } = await startAppWith(false, () => Promise.reject(new Error("down")));

    const response = await request(runningApp.getHttpServer()).get("/api/health/live");

    expect(response.status).toBe(200);
    expect(response.body).toEqual({ status: "ok", service: "api" });
  });
});
