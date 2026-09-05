import { Fragment } from "react";
import { colors } from "@/shared/theme/colors";

/** Only http and https become links. A body containing javascript: or data:
 * stays text — a message is written by whoever is on the other end, and a
 * customer must not be able to put a script behind a CS's click. */
const URL_PATTERN = /https?:\/\/[^\s<>"]+/g;

/** Trailing punctuation belongs to the sentence rather than the address; a
 * link ending in a full stop 404s. Mirrors firstURL in the Go package. */
const TRAILING = /[.,;:!?)\]}'"]+$/;

interface MessageTextProps {
  body: string;
}

/**
 * A message body with its web addresses made clickable.
 *
 * WhatsApp does this for the customer; inside the inbox the same text was
 * inert, so a CS had to select and copy a URL to follow it.
 */
export function MessageText({ body }: MessageTextProps) {
  const parts: React.ReactNode[] = [];
  let cursor = 0;

  for (const match of body.matchAll(URL_PATTERN)) {
    const raw = match[0];
    const start = match.index ?? 0;
    const href = raw.replace(TRAILING, "");
    // Punctuation trimmed off the address is still part of the sentence, so
    // it goes back into the text rather than disappearing.
    const tail = raw.slice(href.length);

    if (start > cursor) parts.push(body.slice(cursor, start));
    parts.push(
      <a
        key={`${start}-${href}`}
        href={href}
        target="_blank"
        // Without noopener the opened page can reach back through
        // window.opener and navigate this tab.
        rel="noopener noreferrer"
        style={{ color: colors.success, textDecoration: "underline" }}
      >
        {href}
      </a>,
    );
    if (tail) parts.push(tail);
    cursor = start + raw.length;
  }

  if (cursor < body.length) parts.push(body.slice(cursor));

  return (
    <>
      {parts.map((part, i) => (
        <Fragment key={i}>{part}</Fragment>
      ))}
    </>
  );
}
