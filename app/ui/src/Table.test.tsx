// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Table, TableBody, TableCell, TableColumnHeader, TableHead, TableRow, TableRowHeader } from "./Table";

describe("Table", () => {
  it("is named by its caption and has column and row headers", () => {
    render(
      <Table caption="Totals by month">
        <TableHead>
          <TableColumnHeader>Period</TableColumnHeader>
          <TableColumnHeader>Total</TableColumnHeader>
        </TableHead>
        <TableBody>
          <TableRow>
            <TableRowHeader>October 2026</TableRowHeader>
            <TableCell numeric>$12.40</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );
    expect(screen.getByRole("table", { name: "Totals by month" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Total" })).toHaveAttribute("scope", "col");
    expect(screen.getByRole("rowheader", { name: "October 2026" })).toHaveAttribute("scope", "row");
    expect(screen.getByRole("cell", { name: "$12.40" })).toHaveClass("tabular-nums");
  });

  it("takes the density down to its cells", () => {
    render(
      <>
        <h3 id="skipped">Skipped lines</h3>
        <Table density="compact" aria-labelledby="skipped">
          <TableHead>
            <TableColumnHeader width="narrow">Line</TableColumnHeader>
          </TableHead>
          <TableBody>
            <TableRow>
              <TableCell>3</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </>,
    );
    expect(screen.getByRole("table", { name: "Skipped lines" })).toHaveClass("table-fixed", "text-sm");
    expect(screen.getByRole("columnheader", { name: "Line" })).toHaveClass("w-16", "font-extrabold");
    expect(screen.getByRole("cell", { name: "3" })).toHaveClass("align-top");
  });
});
