import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ProvisionJob } from "@/domain/entities/Provisioning";
import {
  PICKER_ROWS,
  choosePageSize,
  times,
  visibleRows,
} from "@/test/pagination";
import { ProvisionHistoryModal } from "./ProvisionHistoryModal";

describe("ProvisionHistoryModal", () => {
  it("lets the page-size picker change how many jobs show", async () => {
    const jobs: ProvisionJob[] = times(PICKER_ROWS, (i) => ({
      id: `job-${i}`,
      ontId: "ont-1",
      status: "success",
      createdAt: "2026-10-13T12:00:00Z",
    }));
    render(
      <ProvisionHistoryModal
        open
        ontId="ont-1"
        jobs={jobs}
        loading={false}
        onClose={vi.fn()}
      />,
    );
    expect(visibleRows()).toBe(5);

    await choosePageSize(100);

    expect(visibleRows()).toBe(PICKER_ROWS);
  });
});
