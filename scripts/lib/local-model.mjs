// Reading the output of `ollama list` for the setup check. Pure, so it is tested without Ollama.

/** The Ollama tag a bare name means: `qwen3.5` is `qwen3.5:latest`. */
function withDefaultTag(modelName) {
  return modelName.includes(":") ? modelName : `${modelName}:latest`;
}

/**
 * True when `ollama list` output names the model. The first column of each row is the tag; the
 * header row ("NAME ID SIZE MODIFIED") never matches a real tag.
 */
export function isModelListed(listOutput, modelName) {
  const wanted = withDefaultTag(modelName.trim());
  return listOutput
    .split(/\r?\n/)
    .map((line) => line.trim().split(/\s+/)[0])
    .some((listedName) => listedName === wanted);
}
