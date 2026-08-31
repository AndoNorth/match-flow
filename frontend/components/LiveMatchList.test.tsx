import "@testing-library/jest-dom/vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import * as realtime from "@/lib/gateway/realtime";
import type { EventBody, MatchBody } from "@/lib/gateway/types";
import { LiveMatchList } from "./LiveMatchList";

vi.mock("@/lib/gateway/realtime");

const scheduled: MatchBody = {
  match_id: "s1",
  sport: "football",
  status: "scheduled",
  home_score: 0,
  away_score: 0,
  clock_mins: 0,
};
const live: MatchBody = {
  match_id: "l1",
  sport: "football",
  status: "live",
  home_score: 1,
  away_score: 0,
  clock_mins: 10,
};
const finished: MatchBody = {
  match_id: "f1",
  sport: "football",
  status: "full_time",
  home_score: 2,
  away_score: 2,
  clock_mins: 90,
};

let onUpdate: (event: EventBody) => void;

beforeEach(() => {
  vi.mocked(realtime.subscribeToMatches).mockImplementation(
    (_matchId, _onSnapshot, onUpdateCb) => {
      onUpdate = onUpdateCb;
      return vi.fn();
    },
  );
});

afterEach(cleanup);

describe("LiveMatchList", () => {
  it("renders each initial match under its mapped tab", () => {
    render(<LiveMatchList initialMatches={[scheduled, live, finished]} />);
    expect(screen.getByRole("tab", { name: "Scheduled" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Live" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Finished" })).toBeInTheDocument();
  });

  it("defaults to the Live tab active, showing the live match", () => {
    render(<LiveMatchList initialMatches={[scheduled, live, finished]} />);
    expect(screen.getByText("l1")).toBeInTheDocument();
    expect(screen.queryByText("s1")).not.toBeInTheDocument();
  });

  it("applies a goal update to the matching match's score", () => {
    render(<LiveMatchList initialMatches={[live]} />);
    act(() => {
      onUpdate({
        type: "goal",
        sequence: 1,
        payload: { team: "home", minute: 15 },
        match_id: "l1",
      });
    });
    expect(screen.getByText("2 - 0")).toBeInTheDocument();
  });

  it("ignores a duplicate/lower-sequence update", () => {
    render(<LiveMatchList initialMatches={[live]} />);
    act(() => {
      onUpdate({
        type: "goal",
        sequence: 1,
        payload: { team: "home", minute: 15 },
        match_id: "l1",
      });
    });
    expect(screen.getByText("2 - 0")).toBeInTheDocument();
    act(() => {
      onUpdate({
        type: "goal",
        sequence: 1,
        payload: { team: "home", minute: 15 },
        match_id: "l1",
      });
    });
    expect(screen.getByText("2 - 0")).toBeInTheDocument();
    expect(screen.queryByText("3 - 0")).not.toBeInTheDocument();
  });

  it("ignores an update for a match_id it doesn't know about", () => {
    render(<LiveMatchList initialMatches={[live]} />);
    act(() => {
      onUpdate({
        type: "goal",
        sequence: 1,
        payload: { team: "home" },
        match_id: "unknown",
      });
    });
    expect(screen.getByText("1 - 0")).toBeInTheDocument();
  });
});
