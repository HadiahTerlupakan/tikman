import { afterEach, vi } from "vitest";
import { cleanup } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

afterEach(() => {
  cleanup();
});

global.matchMedia = vi.fn().mockImplementation((query) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: vi.fn(),
  removeListener: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  dispatchEvent: vi.fn(),
}));

// jsdom resolves a computed style by matching every injected CSS rule against
// the element, and its selector parser rejects one of the rules antd emits for
// Select options. rc-motion asks for computed styles during React's commit
// phase, so the exception escapes there and unmounts the whole tree: any
// component that renders a Select inside a Modal or a Collapse becomes
// untestable. Fall back to an empty declaration instead of failing the render;
// jsdom has no layout, so no assertion depends on the values either way.
const computeStyle = window.getComputedStyle.bind(window);
const emptyStyle = document.createElement("div").style;
window.getComputedStyle = ((
  element: Element,
  pseudoElement?: string | null,
) => {
  try {
    return computeStyle(element, pseudoElement);
  } catch {
    return emptyStyle;
  }
}) as typeof window.getComputedStyle;
