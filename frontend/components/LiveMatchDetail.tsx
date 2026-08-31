"use client";

import { useEffect, useRef, useState } from "react";
import { subscribeToMatches } from "@/lib/gateway/realtime";
import { reduce } from "@/lib/gateway/reduce";
import type { EventBody, MatchBody } from "@/lib/gateway/types";
import { ConnectionIndicator } from "./ConnectionIndicator";

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
  const lastSequence = useRef(
    initialEvents.reduce((max, event) => Math.max(max, event.sequence), 0),
  );

  // match.match_id is stable for the life of this component - the
  // detail page always renders one fixed match.
  // biome-ignore lint/correctness/useExhaustiveDependencies: match.match_id is stable for this component's lifetime.
  useEffect(() => {
    const unsubscribe = subscribeToMatches(
      match.match_id,
      (snapshot) => setMatch(snapshot as MatchBody),
      (event) => {
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
      <ConnectionIndicator connected={connected} />
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
            <span
              data-testid="home-score"
              className="text-4xl font-extrabold tabular-nums"
            >
              {match.home_score}
            </span>
            <span className="text-lg opacity-60 self-center">
              {match.clock_mins}'
            </span>
            <span
              data-testid="away-score"
              className="text-4xl font-extrabold tabular-nums"
            >
              {match.away_score}
            </span>
          </div>
          <ul className="timeline timeline-vertical timeline-compact text-xs mt-2">
            {events.map((event) => (
              <li key={event.sequence}>{event.type}</li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  );
}
