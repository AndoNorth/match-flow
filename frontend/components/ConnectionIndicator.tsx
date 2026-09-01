"use client";

export function ConnectionIndicator({ connected }: { connected: boolean }) {
  if (connected) {
    return null;
  }

  return (
    <div role="status" className="badge badge-warning badge-sm">
      reconnecting...
    </div>
  );
}
