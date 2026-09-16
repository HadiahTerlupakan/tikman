import { describe, expect, it } from "vitest";
import { FIBER_LABELS, NODE_LABELS } from "./mappingLabels";

describe("labels", () => {
  it("names every kind of node in Indonesian", () => {
    expect(NODE_LABELS.server).toBe("Server / OLT");
    expect(NODE_LABELS.odc).toBe("ODC");
    expect(NODE_LABELS.odp).toBe("ODP");
    expect(NODE_LABELS.ont).toBe("ONT");
  });

  it("names every kind of cable, cascades included", () => {
    expect(FIBER_LABELS.feeder).toBe("Feeder");
    expect(FIBER_LABELS.distribution).toBe("Distribusi");
    expect(FIBER_LABELS.drop).toBe("Drop");
    expect(FIBER_LABELS.odp_to_odp).toBe("ODP ke ODP");
    expect(FIBER_LABELS.odc_to_odc).toBe("ODC ke ODC");
  });
});
