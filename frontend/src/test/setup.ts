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
// the element, and nwsapi rejects one of the rules antd emits for Select
// options ("*+.ant-select-item-option-selected:not(...))" is not a valid
// selector). rc-motion asks for computed styles during React's commit phase,
// so the exception escapes there and unmounts the whole tree: any component
// that renders a Select inside a Modal or a Collapse becomes untestable. Fall
// back to an empty declaration for that one failure; jsdom has no layout, so
// no assertion depends on the values either way. Every other exception — a
// TypeError from a component calling getComputedStyle(null), say — is a real
// defect and still fails the test.
const computeStyle = window.getComputedStyle.bind(window);
const emptyStyle = document.createElement("div").style;
const isSelectorParseFailure = (error: unknown) =>
  error instanceof DOMException && error.name === "SyntaxError";

window.getComputedStyle = ((
  element: Element,
  pseudoElement?: string | null,
) => {
  try {
    return computeStyle(element, pseudoElement);
  } catch (error) {
    if (!isSelectorParseFailure(error)) {
      throw error;
    }
    return emptyStyle;
  }
}) as typeof window.getComputedStyle;
