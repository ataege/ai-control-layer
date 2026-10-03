// The preview of an export page on screen. A JSON page is indented so it can be read; the download
// never uses this: it saves the body exactly as the server sent it.

export const PREVIEW_CHARACTERS = 1_800;

/** The first part of a page for reading: indented when it is JSON, cut with an ellipsis when long. */
export function previewOfPage(body: string, limit: number = PREVIEW_CHARACTERS): string {
  let readable = body;
  try {
    readable = JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    // Not JSON (a CSV body): show it as it is.
  }
  return readable.length > limit ? `${readable.slice(0, limit)}\n…` : readable;
}
