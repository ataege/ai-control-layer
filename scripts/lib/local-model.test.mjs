// Tests of the setup check's reading of `ollama list` output.
import assert from "node:assert/strict";
import { test } from "node:test";

import { isModelListed } from "./local-model.mjs";

const LIST_OUTPUT = [
  "NAME            ID              SIZE      MODIFIED",
  "qwen3.5:4b      2a654d98e6fb    3.4 GB    2 days ago",
  "llama3.2:latest a80c4f17acd5    2.0 GB    3 weeks ago",
].join("\n");

test("finds the exact tag in the first column", () => {
  assert.equal(isModelListed(LIST_OUTPUT, "qwen3.5:4b"), true);
});

test("does not match a different tag, a prefix or a name in another column", () => {
  assert.equal(isModelListed(LIST_OUTPUT, "qwen3.5:8b"), false);
  assert.equal(isModelListed(LIST_OUTPUT, "qwen3.5"), false);
  assert.equal(isModelListed(LIST_OUTPUT, "qwen3"), false);
  assert.equal(isModelListed(LIST_OUTPUT, "2a654d98e6fb"), false);
  assert.equal(isModelListed(LIST_OUTPUT, "NAME"), false);
});

test("treats a bare name as its latest tag", () => {
  assert.equal(isModelListed(LIST_OUTPUT, "llama3.2"), true);
  assert.equal(isModelListed(LIST_OUTPUT, "qwen3.5"), false);
});

test("finds nothing in an empty list or Windows line endings with no rows", () => {
  assert.equal(isModelListed("", "qwen3.5:4b"), false);
  assert.equal(isModelListed("NAME ID SIZE MODIFIED\r\n", "qwen3.5:4b"), false);
  assert.equal(isModelListed("qwen3.5:4b  2a654d98e6fb  3.4 GB  now\r\n", "qwen3.5:4b"), true);
});
