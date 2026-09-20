/**
 * Saves a blob to disk under the given name, through the standard
 * object-URL-and-anchor trick: there is no direct "save this blob" browser
 * API, and the anchor never needs to be visible or stay in the document to
 * work.
 */
export function downloadFile(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}
