import { Logger } from "@nestjs/common";
import type { DataSource } from "typeorm";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  DatabaseInitializerService,
  describeConnectionFailure,
} from "./database-initializer.service.js";

function createDataSourceStub() {
  return {
    isInitialized: false,
    initialize: vi.fn<() => Promise<void>>(),
    destroy: vi.fn<() => Promise<void>>().mockResolvedValue(undefined),
  };
}

function createService(dataSourceStub: ReturnType<typeof createDataSourceStub>) {
  return new DatabaseInitializerService(dataSourceStub as unknown as DataSource);
}

// Lets an attempt started by a timer or by bootstrap run to its next await point.
async function settlePendingAttempt(): Promise<void> {
  await vi.advanceTimersByTimeAsync(0);
}

describe("DatabaseInitializerService", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Retry warnings and connection notices are expected here.
    vi.spyOn(Logger.prototype, "warn").mockImplementation(() => undefined);
    vi.spyOn(Logger.prototype, "log").mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("retries with a doubling delay and stops once connected", async () => {
    const dataSourceStub = createDataSourceStub();
    const connectionRefused = Object.assign(new Error("refused"), { code: "ECONNREFUSED" });
    dataSourceStub.initialize
      .mockRejectedValueOnce(connectionRefused)
      .mockRejectedValueOnce(connectionRefused)
      .mockImplementationOnce(() => {
        dataSourceStub.isInitialized = true;
        return Promise.resolve();
      });
    const service = createService(dataSourceStub);

    service.onApplicationBootstrap();
    await settlePendingAttempt();
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(999);
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(2);

    await vi.advanceTimersByTimeAsync(1_999);
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(1);
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(3);

    await vi.advanceTimersByTimeAsync(60_000);
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(3);
  });

  it("caps the retry delay at 15 seconds", async () => {
    const dataSourceStub = createDataSourceStub();
    dataSourceStub.initialize.mockRejectedValue(new Error("still down"));
    const service = createService(dataSourceStub);

    service.onApplicationBootstrap();
    // Delays 1s, 2s, 4s and 8s have elapsed after 15s: five attempts so far, the next delay is capped.
    await vi.advanceTimersByTimeAsync(15_000);
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(5);

    await vi.advanceTimersByTimeAsync(14_999);
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(5);
    await vi.advanceTimersByTimeAsync(1);
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(6);

    await vi.advanceTimersByTimeAsync(15_000);
    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(7);

    await service.onApplicationShutdown();
  });

  it("cancels a pending retry on shutdown", async () => {
    const dataSourceStub = createDataSourceStub();
    dataSourceStub.initialize.mockRejectedValue(new Error("down"));
    const service = createService(dataSourceStub);

    service.onApplicationBootstrap();
    await settlePendingAttempt();
    await service.onApplicationShutdown();
    await vi.advanceTimersByTimeAsync(60_000);

    expect(dataSourceStub.initialize).toHaveBeenCalledTimes(1);
    expect(dataSourceStub.destroy).not.toHaveBeenCalled();
  });

  it("waits for an in-flight attempt on shutdown and then closes the connection once", async () => {
    const dataSourceStub = createDataSourceStub();
    let finishConnecting: () => void = () => undefined;
    dataSourceStub.initialize.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          finishConnecting = () => {
            dataSourceStub.isInitialized = true;
            resolve();
          };
        }),
    );
    const service = createService(dataSourceStub);

    service.onApplicationBootstrap();
    let shutdownFinished = false;
    const shutdown = service.onApplicationShutdown().then(() => {
      shutdownFinished = true;
    });
    await settlePendingAttempt();
    expect(shutdownFinished).toBe(false);
    expect(dataSourceStub.destroy).not.toHaveBeenCalled();

    finishConnecting();
    await shutdown;

    expect(dataSourceStub.destroy).toHaveBeenCalledTimes(1);
  });
});

describe("describeConnectionFailure", () => {
  it("keeps a plain driver code and hides everything else", () => {
    expect(describeConnectionFailure({ code: "ECONNREFUSED" })).toBe("ECONNREFUSED");
    expect(describeConnectionFailure({ code: "28P01" })).toBe("28P01");

    expect(describeConnectionFailure(new Error("connect to db-host failed"))).toBe(
      "connection_failed",
    );
    expect(describeConnectionFailure({ code: 57 })).toBe("connection_failed");
    expect(describeConnectionFailure({ code: "postgres://starter:secret@db-host:5432" })).toBe(
      "connection_failed",
    );
    expect(describeConnectionFailure(null)).toBe("connection_failed");
  });
});
