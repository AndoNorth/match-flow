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
  home_score: 0,
  away_score: 0,
  clock_mins: 10,
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
    expect(screen.getByText("0 - 0")).toBeInTheDocument();
    expect(screen.getByText("kickoff")).toBeInTheDocument();
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
    expect(screen.getByText("1 - 0")).toBeInTheDocument();
    expect(screen.getByText("goal")).toBeInTheDocument();
  });

  it("shows the reconnecting indicator when the connection drops", () => {
    render(<LiveMatchDetail initialMatch={match} initialEvents={[]} />);
    act(() => {
      onConnectionChange(false);
    });
    expect(screen.getByRole("status")).toHaveTextContent("reconnecting");
  });
});
