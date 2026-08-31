import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Navbar } from "./Navbar";

describe("Navbar", () => {
  it("shows a success live-feed badge when connected", () => {
    render(<Navbar connected={true} />);
    expect(screen.getByTestId("live-feed-badge")).toHaveTextContent(
      "live feed",
    );
    expect(screen.getByTestId("live-feed-badge")).toHaveClass("badge-success");
  });

  it("shows a warning reconnecting badge when not connected", () => {
    render(<Navbar connected={false} />);
    expect(screen.getByTestId("live-feed-badge")).toHaveTextContent(
      "reconnecting",
    );
    expect(screen.getByTestId("live-feed-badge")).toHaveClass("badge-warning");
  });
});
