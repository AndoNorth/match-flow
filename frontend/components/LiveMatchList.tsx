"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { getMatch } from "@/lib/gateway/client";
import { subscribeToMatches } from "@/lib/gateway/realtime";
import { liveClockMins, reduce } from "@/lib/gateway/reduce";
import type { EventBody, MatchBody } from "@/lib/gateway/types";
import { ConnectionIndicator } from "./ConnectionIndicator";
import { Navbar } from "./Navbar";

type Tab = "Scheduled" | "Live" | "Finished";
const TABS: Tab[] = ["Scheduled", "Live", "Finished"];

function tabFor(status: MatchBody["status"]): Tab {
  if (status === "scheduled") return "Scheduled";
  if (status === "live" || status === "half_time") return "Live";
  return "Finished";
}

type TimeFilter = "all" | "1h" | "24h";
// null means no cutoff - every match passes.
const TIME_FILTER_WINDOW_MS: Record<TimeFilter, number | null> = {
  all: null,
  "1h": 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
};

function withinTimeFilter(match: MatchBody, filter: TimeFilter): boolean {
  const windowMs = TIME_FILTER_WINDOW_MS[filter];
  if (windowMs === null) return true;
  return Date.now() - new Date(match.created_at).getTime() <= windowMs;
}

function toMap(matches: MatchBody[]): Record<string, MatchBody> {
  return Object.fromEntries(matches.map((m) => [m.match_id, m]));
}

export function LiveMatchList({
  initialMatches,
}: {
  initialMatches: MatchBody[];
}) {
  const [matches, setMatches] = useState<Record<string, MatchBody>>(() =>
    toMap(initialMatches),
  );
  const [activeTab, setActiveTab] = useState<Tab>("Live");
  const [timeFilter, setTimeFilter] = useState<TimeFilter>("all");
  const [connected, setConnected] = useState(true);
  const lastSequence = useRef<Record<string, number>>({});
  // Guards against firing getMatch more than once for the same
  // newly-seen match while its fetch is still in flight - later
  // events for it arrive well before a slow fetch resolves.
  const fetchingMatches = useRef<Set<string>>(new Set());
  const [now, setNow] = useState(() => Date.now());

  // Ticks the live-inferred clock forward every second between real
  // events - see liveClockMins for why this doesn't need a server push.
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    const unsubscribe = subscribeToMatches(
      undefined,
      (snapshot) => {
        const list = (snapshot as { matches: MatchBody[] }).matches;
        setMatches(toMap(list));
      },
      (event: EventBody) => {
        // odds_update is never persisted by Match Service - see the
        // matching comment in LiveMatchDetail.tsx. Not user-visible here
        // (no timeline on this page), but skipped for the same reason:
        // it's not a real match-state event.
        if (event.type === "odds_update") return;
        const matchId = event.match_id;
        if (!matchId) return;
        if (event.sequence <= (lastSequence.current[matchId] ?? 0)) return;
        lastSequence.current[matchId] = event.sequence;
        setMatches((prev) => {
          const current = prev[matchId];
          if (current) return { ...prev, [matchId]: reduce(current, event) };
          // A match started after this page's SSE connection opened -
          // the Gateway only sends a full snapshot once, at connect
          // time, so a match not in it has to be fetched directly
          // rather than left to appear on the next page refresh.
          if (!fetchingMatches.current.has(matchId)) {
            fetchingMatches.current.add(matchId);
            getMatch(matchId)
              .then((match) => {
                fetchingMatches.current.delete(matchId);
                setMatches((p) => ({ ...p, [matchId]: match }));
              })
              .catch(() => fetchingMatches.current.delete(matchId));
          }
          return prev;
        });
      },
      setConnected,
    );
    return unsubscribe;
  }, []);

  const inRange = Object.values(matches).filter((m) =>
    withinTimeFilter(m, timeFilter),
  );
  const countFor = (tab: Tab) =>
    inRange.filter((m) => tabFor(m.status) === tab).length;
  const visible = inRange.filter((m) => tabFor(m.status) === activeTab);

  return (
    <div>
      <Navbar connected={connected} />
      <ConnectionIndicator connected={connected} />
      <div className="flex items-center justify-between mb-4 gap-4">
        <div role="tablist" className="tabs tabs-box">
          {TABS.map((tab) => (
            <button
              key={tab}
              type="button"
              role="tab"
              className={`tab ${tab === activeTab ? "tab-active" : ""}`}
              onClick={() => setActiveTab(tab)}
            >
              {tab} ({countFor(tab)})
            </button>
          ))}
        </div>
        <select
          aria-label="Time range"
          className="select select-sm w-auto"
          value={timeFilter}
          onChange={(e) => setTimeFilter(e.target.value as TimeFilter)}
        >
          <option value="all">All time</option>
          <option value="1h">Last hour</option>
          <option value="24h">Last 24h</option>
        </select>
      </div>
      <div className="flex flex-col gap-2">
        {visible.map((match) => (
          <Link
            key={match.match_id}
            href={`/matches/${match.match_id}`}
            className="card card-border bg-base-100"
          >
            <div className="card-body p-4 flex-row items-center justify-between gap-4">
              <div className="flex items-center gap-3 min-w-0">
                <span className="badge badge-sm">{match.status}</span>
                <span className="truncate">
                  {match.home_team} vs {match.away_team}
                </span>
              </div>
              <div className="flex items-center gap-4 shrink-0">
                <span className="opacity-60 tabular-nums text-sm">
                  {liveClockMins(match, now)}'
                </span>
                <span className="text-xl font-bold tabular-nums">
                  {match.home_score} - {match.away_score}
                </span>
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
