import { describe, expect, it } from "vitest";
import type { MikrotikRouter } from "@/domain/entities";
import { rackSlots } from "./rackOrder";

const router = (name: string, rackPosition: number | null): MikrotikRouter =>
  ({ id: name, name, rackPosition }) as MikrotikRouter;

const shape = (routers: MikrotikRouter[]) =>
  rackSlots(routers).map((s) =>
    s.kind === "unit" ? `${s.u}:${s.item.name}` : `${s.u}:blank`,
  );

describe("rackSlots", () => {
  it("fills gaps with blanks and puts unpositioned routers after the highest U", () => {
    expect(shape([router("c", null), router("b", 3), router("a", 1)])).toEqual([
      "1:a",
      "2:blank",
      "3:b",
      "4:c",
    ]);
  });

  it("orders routers by name from U1 when none is positioned", () => {
    expect(shape([router("b", null), router("a", null)])).toEqual([
      "1:a",
      "2:b",
    ]);
  });

  it("moves the second router claiming a U to the end of the unpositioned list", () => {
    expect(
      shape([
        router("b", 2),
        router("a", 2),
        router("z", null),
        router("m", null),
      ]),
    ).toEqual(["1:blank", "2:a", "3:m", "4:z", "5:b"]);
  });
});
