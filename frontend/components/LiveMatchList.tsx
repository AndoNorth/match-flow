"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { subscribeToMatches } from "@/lib/gateway/realtime";
import { reduce } from "@/lib/gateway/reduce";
import type { EventBody, MatchBody } from "@/lib/gateway/types";
import { ConnectionIndicator } from "./ConnectionIndicator";

type Tab = "Scheduled" | "Live" | "Finished";
const TABS: Tab[] = ["Scheduled", "Live", "Finished"];

function tabFor(status: MatchBody["status"]): Tab {
  if (status === "scheduled") return "Scheduled";
  if (status === "live" || status === "half_time") return "Live";
  return "Finished";
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
  const [connected, setConnected] = useState(true);
  const lastSequence = useRef<Record<string, number>>({});

  useEffect(() => {
    const unsubscribe = subscribeToMatches(
      undefined,
      (snapshot) => {
        const list = (snapshot as { matches: MatchBody[] }).matches;
        setMatches(toMap(list));
      },
      (event: EventBody) => {
        const matchId = event.match_id;
        if (!matchId) return;
        if (event.sequence <= (lastSequence.current[matchId] ?? 0)) return;
        setMatches((prev) => {
          const current = prev[matchId];
          if (!current) return prev;
          lastSequence.current[matchId] = event.sequence;
          return { ...prev, [matchId]: reduce(current, event) };
        });
      },
      setConnected,
    );
    return unsubscribe;
  }, []);

  const visible = Object.values(matches).filter(
    (m) => tabFor(m.status) === activeTab,
  );

  return (
    <div>
      <ConnectionIndicator connected={connected} />
      <div role="tablist" className="tabs tabs-box mb-4">
        {TABS.map((tab) => (
          <button
            key={tab}
            type="button"
            role="tab"
            className={`tab ${tab === activeTab ? "tab-active" : ""}`}
            onClick={() => setActiveTab(tab)}
          >
            {tab}
          </button>
        ))}
      </div>
      <div className="flex flex-col gap-2">
        {visible.map((match) => (
          <Link
            key={match.match_id}
            href={`/matches/${match.match_id}`}
            className="card card-border bg-base-100"
          >
            <div className="card-body p-4 flex-row items-center justify-between">
              <span>{match.match_id}</span>
              <span className="text-xl font-bold tabular-nums">
                {match.home_score} - {match.away_score}
              </span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
