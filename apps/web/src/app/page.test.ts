import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

// The task form is a client component with its own tests' needs (router, client); the page's own text
// is what is under test here.
vi.mock("../components/task-form", () => ({
  TaskForm: () => createElement("div", { "data-testid": "task-form" }),
}));

const { default: HomePage } = await import("./page");

describe("the home page", () => {
  const html = renderToStaticMarkup(createElement(HomePage));

  it("shows the task form", () => {
    expect(html).toContain('data-testid="task-form"');
  });

  it("presents no sample or mock run data", () => {
    // These were the mock timeline's rows and heading; none may come back as if it were a real run.
    for (const sample of [
      "Sample Data",
      "Mock execution",
      "10:00 AM",
      "Task requested by user",
      "Invoice data extracted",
      "Task completed successfully",
    ]) {
      expect(html).not.toContain(sample);
    }
  });

  it("says nothing about the starter it replaced", () => {
    expect(html).not.toMatch(/starter|infrastructure and reusable|No tables yet/i);
  });

  it("states what is simulated and synthetic, from the label module", () => {
    expect(html).toContain("a database record, no email is sent");
    expect(html).toContain("Synthetic records");
  });

  it("does not claim the controls narrow a rejected request for the operator", () => {
    expect(html).toContain("nothing narrows it for you");
  });
});
