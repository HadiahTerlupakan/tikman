import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { RouterBgpEvent } from "@/domain/entities";
import {
  PICKER_ROWS,
  choosePageSize,
  times,
  visibleRows,
} from "@/test/pagination";
import { BgpEventList } from "./BgpEventList";

describe("BgpEventList", () => {
  it("lets the page-size picker change how many events show", async () => {
    const events: RouterBgpEvent[] = times(PICKER_ROWS, (i) => ({
      id: `event-${i}`,
      routerId: "router-1",
      sessionName: `peer-${i}`,
      fromState: "established",
      toState: "down",
      occurredAt: "2026-10-13T12:00:00Z",
    }));
    render(<BgpEventList events={events} />);
    expect(visibleRows()).toBe(20);

    await choosePageSize(100);

    expect(visibleRows()).toBe(PICKER_ROWS);
  });
});
