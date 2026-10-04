import { describe, expect, it } from "vitest";

import { countOf, pluralize } from "./plural";

describe("pluralize", () => {
  it("takes the singular for exactly 1 and the plural for 0, 2 and everything else", () => {
    expect(pluralize(1, "has", "have")).toBe("has");
    expect(pluralize(0, "has", "have")).toBe("have");
    expect(pluralize(2, "has", "have")).toBe("have");
    expect(pluralize(11, "has", "have")).toBe("have");
  });
});

describe("countOf", () => {
  it("says '1 message', '0 messages' and '2 messages'", () => {
    expect(countOf(0, "message")).toBe("0 messages");
    expect(countOf(1, "message")).toBe("1 message");
    expect(countOf(2, "message")).toBe("2 messages");
  });

  it("uses a given plural for a word that does not just add an s", () => {
    expect(countOf(1, "query", "queries")).toBe("1 query");
    expect(countOf(2, "query", "queries")).toBe("2 queries");
    expect(countOf(0, "query", "queries")).toBe("0 queries");
  });
});
