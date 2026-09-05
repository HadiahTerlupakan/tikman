import { useEffect, useMemo, useRef, useState } from "react";
import { CsRepository } from "@/infrastructure/repositories";
import { SEARCH_DEBOUNCE_MS } from "@/shared/config/limits";
import type { CsLinkPreview } from "@/domain/entities";

const csRepository = new CsRepository();

/** Mirrors firstURL in the Go package: only http and https, and trailing
 * sentence punctuation is not part of the address. Kept in step deliberately —
 * a draft the browser thinks holds a link but the server does not would ask
 * for a card on every keystroke. */
const URL_PATTERN = /https?:\/\/[^\s<>"]+/;

function firstLink(text: string): string | null {
  const found = text.match(URL_PATTERN)?.[0];
  return found ? found.replace(/[.,;:!?)\]}'"]+$/, "") : null;
}

export interface LinkPreviewState {
  preview: CsLinkPreview | null;
  /** Hides the card for the link currently in the draft. */
  dismiss: () => void;
}

/**
 * The card the composer shows while a CS types a link.
 *
 * Display only: what the customer receives is resolved again by the wa
 * process at send time, so this cannot change the message — only what the CS
 * sees before sending it.
 *
 * The draft is watched rather than every keystroke sent: the request fires
 * only once the text actually contains a link, and only after typing settles.
 */
export function useLinkPreview(draft: string): LinkPreviewState {
  const link = useMemo(() => firstLink(draft), [draft]);
  const [preview, setPreview] = useState<CsLinkPreview | null>(null);
  const dismissed = useRef<string | null>(null);

  useEffect(() => {
    if (!link || dismissed.current === link) {
      setPreview(null);
      return;
    }

    // A draft in progress changes on every keystroke; only the settled one is
    // worth a request, and the stale flag drops answers that arrive after the
    // link has moved on.
    let stale = false;
    const timer = setTimeout(() => {
      csRepository
        .getLinkPreview(draft)
        .then((found) => {
          if (!stale) setPreview(found);
        })
        .catch(() => {
          // A preview is decoration. A failure leaves the composer as it was.
          if (!stale) setPreview(null);
        });
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      stale = true;
      clearTimeout(timer);
    };
    // draft is read inside, but only `link` should retrigger: typing more
    // words around an unchanged URL must not refetch it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [link]);

  return {
    preview,
    dismiss: () => {
      dismissed.current = link;
      setPreview(null);
    },
  };
}
