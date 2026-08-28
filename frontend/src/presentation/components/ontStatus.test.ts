import { describe, expect, it } from "vitest";
import { OntStatus } from "@/domain/entities";
import { ONT_STATUSES, ontStatusColor, ontStatusLabel } from "./ontStatus";

describe("ont status presentation", () => {
  // The table shouted the raw enum, so an ONU in dying gasp read DYING_GASP.
  it("reads as words, not as a database name", () => {
    expect(ontStatusLabel(OntStatus.DYING_GASP)).toBe("Dying gasp");
    expect(ontStatusLabel(OntStatus.ONLINE)).toBe("Online");
  });

  it("falls back to Unknown for a status it does not know", () => {
    expect(ontStatusLabel(undefined)).toBe("Unknown");
    expect(ontStatusColor(undefined)).toBe("default");
  });

  // A dying gasp is the ONU's last message before it loses power; it has to
  // stand out from an ONU that is merely offline.
  it("marks a dying gasp as an error and offline as neutral", () => {
    expect(ontStatusColor(OntStatus.DYING_GASP)).toBe("error");
    expect(ontStatusColor(OntStatus.OFFLINE)).toBe("default");
  });

  it("covers every status the filter offers", () => {
    expect(ONT_STATUSES.map((s) => s.value)).toEqual(Object.values(OntStatus));
  });
});
