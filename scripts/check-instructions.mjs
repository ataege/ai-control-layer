#!/usr/bin/env node
// Validates the team instruction files and the Claude Code project agents.
// Usage: node scripts/check-instructions.mjs [rootDirectory]
// The root can also be set with INSTRUCTIONS_ROOT; it defaults to the repository root.

import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const rootDirectory = resolve(
  process.argv[2] ?? process.env.INSTRUCTIONS_ROOT ?? join(scriptDirectory, ".."),
);

const INSTRUCTION_FILE_NAMES = ["AGENTS.md", "CLAUDE.md"];
const AGENTS_DIRECTORY = join(".claude", "agents");
// Build-time marker that must be replaced by the real verification statement.
const VERIFICATION_PLACEHOLDER = "VERIFICATION-STATUS: not yet recorded";
const EXPECTED_AGENT_NAMES = [
  "frontend",
  "go",
  "infrastructure",
  "integration",
  "nestjs",
  "reviewer",
];
const REQUIRED_KEYS = ["name", "description", "tools", "model"];
const REVIEWER_TOOLS = ["Glob", "Grep", "Read"];
// Tool names an agent may list; anything else is a typo or an unreviewed capability.
const KNOWN_TOOLS = new Set(["Read", "Grep", "Glob", "Edit", "Write", "Bash"]);
// Keys that widen what an agent may do without a prompt, or load extra machinery.
const FORBIDDEN_KEY_PATTERN =
  /^(permissionMode|permission_mode|hooks|mcpServers|mcp_servers|experimental|isolation|background|.*bypass.*|.*dangerous.*)$/i;
const FORBIDDEN_VALUE_PATTERN = /bypassPermissions|dangerously|acceptEdits|dontAsk/i;

let failureCount = 0;

/** Prints one check result and records failures. */
function report(passed, label, problems = []) {
  console.log(`${passed ? "PASS" : "FAIL"}  ${label}`);
  for (const problem of problems) {
    console.log(`      - ${problem}`);
  }
  if (!passed) {
    failureCount += 1;
  }
}

/** Returns the 1-based number of the first line that differs between two texts. */
function firstDifferingLine(firstText, secondText) {
  const firstLines = firstText.split("\n");
  const secondLines = secondText.split("\n");
  const sharedLength = Math.min(firstLines.length, secondLines.length);
  for (let lineIndex = 0; lineIndex < sharedLength; lineIndex += 1) {
    if (firstLines[lineIndex] !== secondLines[lineIndex]) {
      return lineIndex + 1;
    }
  }
  return sharedLength + 1;
}

/** Finds at-sign imports outside code spans and fenced blocks. */
function findImportLines(markdownText) {
  const importLineNumbers = [];
  let insideFence = false;
  markdownText.split("\n").forEach((line, lineIndex) => {
    if (/^\s*(```|~~~)/.test(line)) {
      insideFence = !insideFence;
      return;
    }
    if (insideFence) {
      return;
    }
    const lineWithoutCodeSpans = line.replace(/`[^`]*`/g, "");
    if (/(^|\s)@[\w./~-]/.test(lineWithoutCodeSpans)) {
      importLineNumbers.push(lineIndex + 1);
    }
  });
  return importLineNumbers;
}

/** Check 1: both instruction files exist, are byte-identical, import-free and verified. */
function checkInstructionFiles() {
  const [agentsPath, claudePath] = INSTRUCTION_FILE_NAMES.map((fileName) =>
    join(rootDirectory, fileName),
  );
  const missingFiles = INSTRUCTION_FILE_NAMES.filter(
    (fileName) => !existsSync(join(rootDirectory, fileName)),
  );
  if (missingFiles.length > 0) {
    report(false, "AGENTS.md and CLAUDE.md exist", [`missing: ${missingFiles.join(", ")}`]);
    return;
  }
  report(true, "AGENTS.md and CLAUDE.md exist");

  const agentsBytes = readFileSync(agentsPath);
  const claudeBytes = readFileSync(claudePath);
  if (agentsBytes.equals(claudeBytes)) {
    report(true, `AGENTS.md and CLAUDE.md are byte-identical (${agentsBytes.length} bytes)`);
  } else {
    const differingLine = firstDifferingLine(
      agentsBytes.toString("utf8"),
      claudeBytes.toString("utf8"),
    );
    report(false, "AGENTS.md and CLAUDE.md are byte-identical", [
      `sizes: AGENTS.md ${agentsBytes.length} bytes, CLAUDE.md ${claudeBytes.length} bytes`,
      `first difference at line ${differingLine}`,
      "fix: move the wanted edit into AGENTS.md, then run `cp AGENTS.md CLAUDE.md`",
    ]);
  }

  const importLineNumbers = findImportLines(agentsBytes.toString("utf8"));
  report(
    importLineNumbers.length === 0,
    "AGENTS.md contains no at-sign imports",
    importLineNumbers.map(
      (lineNumber) => `line ${lineNumber}: wrap the at-sign token in backticks or remove it`,
    ),
  );

  const hasPlaceholder = agentsBytes.toString("utf8").includes(VERIFICATION_PLACEHOLDER);
  report(
    !hasPlaceholder,
    "AGENTS.md records the verification status",
    hasPlaceholder ? ["replace the VERIFICATION-STATUS placeholder with the real status"] : [],
  );
}

/**
 * Parses flat `key: value` frontmatter. Returns the fields and body, or a list of problems.
 * Nested YAML is rejected on purpose: agent files here only need scalar values.
 */
function parseAgentFile(fileText) {
  const lines = fileText.split(/\r?\n/);
  if (lines[0] !== "---") {
    return { problems: ["the first line must be exactly `---` (frontmatter start)"] };
  }
  const closingLineIndex = lines.indexOf("---", 1);
  if (closingLineIndex === -1) {
    return { problems: ["frontmatter has no closing `---` line"] };
  }

  const problems = [];
  const fields = new Map();
  for (let lineIndex = 1; lineIndex < closingLineIndex; lineIndex += 1) {
    const line = lines[lineIndex];
    const lineNumber = lineIndex + 1;
    if (line.trim() === "" || line.trimStart().startsWith("#")) {
      continue;
    }
    if (/^\s/.test(line) || /^-(\s|$)/.test(line)) {
      problems.push(
        `line ${lineNumber}: nested or list YAML is not supported; use one \`key: value\` per line (tools as a comma-separated string)`,
      );
      continue;
    }
    const keyValueMatch = /^([A-Za-z_][\w-]*):(?:\s+(.*))?$/.exec(line);
    if (!keyValueMatch) {
      problems.push(`line ${lineNumber}: not a \`key: value\` line`);
      continue;
    }
    const key = keyValueMatch[1];
    let value = (keyValueMatch[2] ?? "").trim();
    const quotedMatch = /^(["'])(.*)\1$/.exec(value);
    if (quotedMatch) {
      value = quotedMatch[2];
    } else if (/^[[{>|&*!%@`]/.test(value)) {
      problems.push(`line ${lineNumber}: value of "${key}" must be a plain or quoted scalar`);
    } else if (/:\s/.test(value) || /\s#/.test(value)) {
      // A plain scalar with ": " or " #" would be read differently by a real YAML parser.
      problems.push(`line ${lineNumber}: quote the value of "${key}" (it contains ": " or " #")`);
    }
    if (fields.has(key)) {
      problems.push(`line ${lineNumber}: duplicate key "${key}"`);
    }
    fields.set(key, value);
  }

  const body = lines.slice(closingLineIndex + 1).join("\n");
  return { fields, body, problems };
}

/** Splits a comma-separated tools value into trimmed names. */
function parseToolList(toolsValue) {
  return toolsValue
    .split(",")
    .map((toolName) => toolName.trim())
    .filter((toolName) => toolName !== "");
}

/** Returns every problem found in one agent file. */
function validateAgentFile(fileName, fileText) {
  const expectedName = fileName.replace(/\.md$/, "");
  const parsed = parseAgentFile(fileText);
  if (!parsed.fields) {
    return parsed.problems;
  }
  const { fields, body } = parsed;
  const problems = [...parsed.problems];

  for (const requiredKey of REQUIRED_KEYS) {
    if (!fields.has(requiredKey) || fields.get(requiredKey) === "") {
      problems.push(`missing required key "${requiredKey}"`);
    }
  }
  for (const [key, value] of fields) {
    if (FORBIDDEN_KEY_PATTERN.test(key)) {
      problems.push(
        `forbidden key "${key}": agents must inherit the session's permission settings`,
      );
    } else if (!REQUIRED_KEYS.includes(key)) {
      // Unknown keys are silently ignored by Claude Code, so a typo would go unnoticed.
      problems.push(`unexpected key "${key}" (allowed: ${REQUIRED_KEYS.join(", ")})`);
    }
    if (key !== "description" && FORBIDDEN_VALUE_PATTERN.test(value)) {
      problems.push(`forbidden value in "${key}": ${value}`);
    }
  }

  const agentName = fields.get("name");
  if (agentName && agentName !== expectedName) {
    problems.push(`name "${agentName}" must equal the file name "${expectedName}"`);
  }
  if (agentName && (agentName.includes(":") || agentName.startsWith("-"))) {
    problems.push(`name "${agentName}" must not contain ":" or start with "-"`);
  }
  if (fields.has("model") && fields.get("model") !== "inherit") {
    problems.push(`model must be "inherit", found "${fields.get("model")}"`);
  }

  const toolNames = parseToolList(fields.get("tools") ?? "");
  const unknownTools = toolNames.filter((toolName) => !KNOWN_TOOLS.has(toolName));
  if (unknownTools.length > 0) {
    problems.push(
      `unknown tool(s): ${unknownTools.join(", ")} (allowed: ${[...KNOWN_TOOLS].join(", ")})`,
    );
  }
  if (new Set(toolNames).size !== toolNames.length) {
    problems.push("tools lists the same tool more than once");
  }
  if (expectedName === "reviewer") {
    const sortedTools = [...toolNames].sort();
    if (sortedTools.join(",") !== REVIEWER_TOOLS.join(",")) {
      problems.push(
        `reviewer must be read-only: tools must be exactly Read, Grep, Glob, found "${toolNames.join(", ")}"`,
      );
    }
  }

  if (body.trim() === "") {
    problems.push("the body (the agent's system prompt) is empty");
  }
  return problems;
}

/** Check 2: the expected agents exist and every agent file is valid. */
function checkAgentFiles() {
  const agentsDirectoryPath = join(rootDirectory, AGENTS_DIRECTORY);
  if (!existsSync(agentsDirectoryPath)) {
    report(false, `${AGENTS_DIRECTORY} exists`, [`not found: ${agentsDirectoryPath}`]);
    return;
  }
  const agentFileNames = readdirSync(agentsDirectoryPath)
    .filter((fileName) => fileName.endsWith(".md"))
    .sort();

  const missingAgentNames = EXPECTED_AGENT_NAMES.filter(
    (agentName) => !agentFileNames.includes(`${agentName}.md`),
  );
  report(
    missingAgentNames.length === 0,
    `the ${EXPECTED_AGENT_NAMES.length} expected agents exist (${EXPECTED_AGENT_NAMES.join(", ")})`,
    missingAgentNames.map((agentName) => `missing: ${join(AGENTS_DIRECTORY, `${agentName}.md`)}`),
  );

  for (const fileName of agentFileNames) {
    const fileText = readFileSync(join(agentsDirectoryPath, fileName), "utf8");
    const problems = validateAgentFile(fileName, fileText);
    report(
      problems.length === 0,
      `${join(AGENTS_DIRECTORY, fileName)} is a valid agent file`,
      problems,
    );
  }
}

console.log(`Checking instructions in ${rootDirectory}`);
checkInstructionFiles();
checkAgentFiles();

if (failureCount > 0) {
  console.log(`\n${failureCount} check(s) failed.`);
  process.exitCode = 1;
} else {
  console.log("\nAll instruction checks passed.");
}
