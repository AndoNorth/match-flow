import "@testing-library/jest-dom/vitest";
import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as realtime from "@/lib/gateway/realtime";
import type { EventBody, MatchBody } from "@/lib/gateway/types";
import { LiveMatchDetail } from "./LiveMatchDetail";

vi.mock("@/lib/gateway/realtime");

const match: MatchBody = {
  match_id: "m1",
  sport: "football",
  status: "live",
  home_team: "Ashford United",
  away_team: "Denbury City",
  home_score: 0,
  away_score: 0,
  clock_mins: 10,
  created_at: "2026-08-31T12:00:00Z",
};
const kickoff: EventBody = { type: "kickoff", sequence: 1, payload: {} };

let onUpdate: (event: EventBody) => void;
let onConnectionChange: (connected: boolean) => void;

beforeEach(() => {
  vi.mocked(realtime.subscribeToMatches).mockImplementation(
    (_matchId, _onSnapshot, onUpdateCb, onConnectionChangeCb) => {
      onUpdate = onUpdateCb;
      onConnectionChange = onConnectionChangeCb as (connected: boolean) => void;
      return vi.fn();
    },
  );
});

describe("LiveMatchDetail", () => {
  it("renders the initial score and timeline", () => {
    render(<LiveMatchDetail initialMatch={match} initialEvents={[kickoff]} />);
    expect(screen.getByTestId("home-score")).toHaveTextContent("0");
    expect(screen.getByTestId("away-score")).toHaveTextContent("0");
    expect(screen.getByText("kickoff")).toBeInTheDocument();
  });

  it("does not render the score twice", () => {
    render(<LiveMatchDetail initialMatch={match} initialEvents={[kickoff]} />);
    expect(screen.getAllByText("0")).toHaveLength(2);
  });

  it("applies a goal update to the score and appends it to the timeline", () => {
    render(<LiveMatchDetail initialMatch={match} initialEvents={[kickoff]} />);
    act(() => {
      onUpdate({
        type: "goal",
        sequence: 2,
        payload: { team: "home", minute: 23 },
      });
    });
    expect(screen.getByTestId("home-score")).toHaveTextContent("1");
    expect(screen.getByTestId("away-score")).toHaveTextContent("0");
    expect(screen.getByText("goal")).toBeInTheDocument();
  });

  it("ignores a duplicate/lower-sequence update", () => {
    render(<LiveMatchDetail initialMatch={match} initialEvents={[kickoff]} />);
    act(() => {
      onUpdate({
        type: "goal",
        sequence: 1,
        payload: { team: "home", minute: 23 },
      });
    });
    expect(screen.getByTestId("home-score")).toHaveTextContent("0");
    expect(screen.getByTestId("away-score")).toHaveTextContent("0");
    expect(screen.queryByText("goal")).not.toBeInTheDocument();
  });

  it("ignores an odds_update event, leaving score and timeline unchanged", () => {
    render(<LiveMatchDetail initialMatch={match} initialEvents={[kickoff]} />);
    act(() => {
      onUpdate({
        type: "odds_update",
        sequence: 2,
        payload: { market: "match_winner", selection: "home", price: 1.5 },
      });
    });
    expect(screen.getByTestId("home-score")).toHaveTextContent("0");
    expect(screen.getByTestId("away-score")).toHaveTextContent("0");
    expect(screen.queryByText("odds_update")).not.toBeInTheDocument();
  });

  it("shows a live-feed badge reflecting connection state", () => {
    render(<LiveMatchDetail initialMatch={match} initialEvents={[]} />);
    expect(screen.getByTestId("live-feed-badge")).toHaveTextContent(
      "live feed",
    );
    act(() => {
      onConnectionChange(false);
    });
    expect(screen.getByTestId("live-feed-badge")).toHaveTextContent(
      "reconnecting",
    );
  });

  it("has a link back to the match list", () => {
    render(<LiveMatchDetail initialMatch={match} initialEvents={[]} />);
    expect(
      screen.getByRole("link", { name: /back to matches/i }),
    ).toHaveAttribute("href", "/");
  });

  it("shows the reconnecting indicator when the connection drops", () => {
    render(<LiveMatchDetail initialMatch={match} initialEvents={[]} />);
    act(() => {
      onConnectionChange(false);
    });
    expect(screen.getByRole("status")).toHaveTextContent("reconnecting");
  });
});
