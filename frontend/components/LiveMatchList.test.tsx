import "@testing-library/jest-dom/vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import * as realtime from "@/lib/gateway/realtime";
import type { EventBody, MatchBody } from "@/lib/gateway/types";
import { LiveMatchList } from "./LiveMatchList";

vi.mock("@/lib/gateway/realtime");

const now = new Date().toISOString();
const twoHoursAgo = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString();

const scheduled: MatchBody = {
  match_id: "s1",
  sport: "football",
  status: "scheduled",
  home_team: "Ashford United",
  away_team: "Denbury City",
  home_score: 0,
  away_score: 0,
  clock_mins: 0,
  created_at: now,
};
const live: MatchBody = {
  match_id: "l1",
  sport: "football",
  status: "live",
  home_team: "Foxwick Athletic",
  away_team: "Greymoor Rovers",
  home_score: 1,
  away_score: 0,
  clock_mins: 10,
  created_at: now,
};
const finished: MatchBody = {
  match_id: "f1",
  sport: "football",
  status: "full_time",
  home_team: "Harrowgate Town",
  away_team: "Ironbridge Wanderers",
  home_score: 2,
  away_score: 2,
  clock_mins: 90,
  created_at: now,
};
const oldFinished: MatchBody = {
  match_id: "f2",
  sport: "football",
  status: "full_time",
  home_team: "Kingswell Albion",
  away_team: "Lambourne FC",
  home_score: 1,
  away_score: 1,
  clock_mins: 90,
  created_at: twoHoursAgo,
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
    expect(screen.getByRole("tab", { name: /Scheduled/ })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Live/ })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Finished/ })).toBeInTheDocument();
  });

  it("shows a count next to each tab", () => {
    render(<LiveMatchList initialMatches={[scheduled, live, finished]} />);
    expect(
      screen.getByRole("tab", { name: "Scheduled (1)" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Live (1)" })).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "Finished (1)" }),
    ).toBeInTheDocument();
  });

  it("only counts and shows matches within the selected time range", () => {
    render(<LiveMatchList initialMatches={[live, finished, oldFinished]} />);
    expect(
      screen.getByRole("tab", { name: "Finished (2)" }),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Time range"), {
      target: { value: "1h" },
    });

    expect(
      screen.getByRole("tab", { name: "Finished (1)" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Finished (1)" }));
    expect(
      screen.getByText("Harrowgate Town vs Ironbridge Wanderers"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Kingswell Albion vs Lambourne FC"),
    ).not.toBeInTheDocument();
  });

  it("defaults to the Live tab active, showing the live match", () => {
    render(<LiveMatchList initialMatches={[scheduled, live, finished]} />);
    expect(
      screen.getByText("Foxwick Athletic vs Greymoor Rovers"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Ashford United vs Denbury City"),
    ).not.toBeInTheDocument();
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

  it("ignores an odds_update event, leaving score unchanged", () => {
    render(<LiveMatchList initialMatches={[live]} />);
    act(() => {
      onUpdate({
        type: "odds_update",
        sequence: 1,
        payload: { market: "match_winner", selection: "home", price: 1.5 },
        match_id: "l1",
      });
    });
    expect(screen.getByText("1 - 0")).toBeInTheDocument();
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
