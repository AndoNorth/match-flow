"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { subscribeToMatches } from "@/lib/gateway/realtime";
import { eventMinute, liveClockMins, reduce } from "@/lib/gateway/reduce";
import type { EventBody, MatchBody } from "@/lib/gateway/types";
import { ConnectionIndicator } from "./ConnectionIndicator";
import { Navbar } from "./Navbar";

export function LiveMatchDetail({
  initialMatch,
  initialEvents,
}: {
  initialMatch: MatchBody;
  initialEvents: EventBody[];
}) {
  const [match, setMatch] = useState(initialMatch);
  const [events, setEvents] = useState(initialEvents);
  const [connected, setConnected] = useState(true);
  const [now, setNow] = useState(() => Date.now());
  const lastSequence = useRef(
    initialEvents.reduce((max, event) => Math.max(max, event.sequence), 0),
  );

  // Ticks the live-inferred clock forward every second between real
  // events - see liveClockMins for why this doesn't need a server push.
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  // match.match_id is stable for the life of this component - the
  // detail page always renders one fixed match.
  // biome-ignore lint/correctness/useExhaustiveDependencies: match.match_id is stable for this component's lifetime.
  useEffect(() => {
    const unsubscribe = subscribeToMatches(
      match.match_id,
      (snapshot) => setMatch(snapshot as MatchBody),
      (event) => {
        // odds_update is never persisted by Match Service
        // (services/match-service/internal/eventstream drops it before a
        // Rule lookup ever happens) - the Gateway's SSE stream has no such
        // filter, so without this check it would flash into the timeline
        // live and then vanish on the next server-rendered load, since
        // GET /matches/{id}/events never had it either.
        if (event.type === "odds_update") return;
        if (event.sequence <= lastSequence.current) return;
        lastSequence.current = event.sequence;
        setMatch((prev) => reduce(prev, event));
        setEvents((prev) => [...prev, event]);
      },
      setConnected,
    );
    return unsubscribe;
  }, []);

  return (
    <div>
      <Navbar connected={connected} />
      <ConnectionIndicator connected={connected} />
      <Link href="/" className="btn btn-sm btn-ghost mb-2">
        ← Back to matches
      </Link>
      <div className="card card-border bg-base-200">
        <div className="card-body p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="badge">{match.status}</span>
            <span className="text-xs opacity-60 font-mono">
              {match.match_id}
            </span>
          </div>
          <div
            data-testid="score"
            className="flex items-center justify-center gap-6 my-3"
          >
            <div className="flex flex-col items-center gap-1">
              <span className="text-sm font-semibold truncate max-w-48">
                {match.home_team}
              </span>
              <span
                data-testid="home-score"
                className="text-4xl font-extrabold tabular-nums"
              >
                {match.home_score}
              </span>
            </div>
            <span className="text-lg opacity-60 self-center">
              {liveClockMins(match, now)}'
            </span>
            <div className="flex flex-col items-center gap-1">
              <span className="text-sm font-semibold truncate max-w-48">
                {match.away_team}
              </span>
              <span
                data-testid="away-score"
                className="text-4xl font-extrabold tabular-nums"
              >
                {match.away_score}
              </span>
            </div>
          </div>
          <ul className="flex flex-col gap-1 mt-2">
            {events
              .slice()
              .reverse()
              .map((event) => (
                <li
                  key={event.sequence}
                  className="card card-border bg-base-100 flex-row items-center justify-between px-3 py-2 text-sm"
                >
                  <span className="capitalize">{event.type}</span>
                  <span className="opacity-60 tabular-nums">
                    {eventMinute(event) ?? "-"}'
                  </span>
                </li>
              ))}
          </ul>
        </div>
      </div>
    </div>
  );
}
