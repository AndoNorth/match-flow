"use client";

export function Navbar({ connected }: { connected: boolean }) {
  return (
    <div className="navbar bg-base-200 rounded-box mb-4">
      <div className="navbar-start">
        <span className="text-lg font-bold px-2">MatchFlow</span>
      </div>
      <div className="navbar-end">
        <span
          data-testid="live-feed-badge"
          className={`badge badge-soft mr-2 ${connected ? "badge-success" : "badge-warning"}`}
        >
          {connected ? "● live feed" : "● reconnecting"}
        </span>
      </div>
    </div>
  );
}
