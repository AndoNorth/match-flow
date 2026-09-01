import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ConnectionIndicator } from "./ConnectionIndicator";

describe("ConnectionIndicator", () => {
  it("renders nothing when connected", () => {
    render(<ConnectionIndicator connected={true} />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("shows a reconnecting message when not connected", () => {
    render(<ConnectionIndicator connected={false} />);
    expect(screen.getByRole("status")).toHaveTextContent("reconnecting");
  });
});
