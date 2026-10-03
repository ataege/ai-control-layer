// Saves text in the browser as a file, byte for byte: the body is wrapped in a Blob unchanged and
// handed to the browser's download. Browser-only; nothing here runs on the server.

export function saveTextFile(fileName: string, body: string, contentType: string): void {
  const url = URL.createObjectURL(new Blob([body], { type: contentType || "text/plain" }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  anchor.rel = "noopener";
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  // Revoke after the click has been handled.
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
