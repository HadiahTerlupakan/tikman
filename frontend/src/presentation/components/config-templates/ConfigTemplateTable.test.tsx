import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ConfigTemplate } from "@/domain/entities/ConfigTemplate";
import {
  PICKER_ROWS,
  choosePageSize,
  times,
  visibleRows,
} from "@/test/pagination";
import { ConfigTemplateTable } from "./ConfigTemplateTable";

describe("ConfigTemplateTable", () => {
  it("lets the page-size picker change how many templates show", async () => {
    const templates: ConfigTemplate[] = times(PICKER_ROWS, (i) => ({
      id: `template-${i}`,
      name: `Template ${i}`,
      description: "",
      vendor: "ZTE",
      configFields: {},
      isDefault: false,
      createdAt: "2026-10-13T12:00:00Z",
      updatedAt: "2026-10-13T12:00:00Z",
    }));
    render(
      <ConfigTemplateTable
        templates={templates}
        loading={false}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
      />,
    );
    expect(visibleRows()).toBe(10);

    await choosePageSize(100);

    expect(visibleRows()).toBe(PICKER_ROWS);
  });
});
