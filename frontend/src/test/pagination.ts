import { fireEvent, screen } from "@testing-library/react";

/** Picks a page size in an antd Table's size picker. */
export async function choosePageSize(size: number) {
  const picker = document.querySelector(
    ".ant-pagination-options .ant-select-selector",
  );
  if (!picker) throw new Error("the table shows no page-size picker");
  fireEvent.mouseDown(picker);
  fireEvent.click(await screen.findByTitle(`${size} / page`));
}

/** How many data rows the table shows on the current page. */
export function visibleRows(): number {
  return document.querySelectorAll("tbody tr.ant-table-row").length;
}

/** Enough rows for antd to show its size picker, which needs more than 50. */
export const PICKER_ROWS = 60;

export function times<T>(n: number, make: (i: number) => T): T[] {
  return Array.from({ length: n }, (_, i) => make(i));
}
