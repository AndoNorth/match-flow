import type { EventBody, MatchBody } from "./types";

type ListSnapshot = { matches: MatchBody[] };

function baseUrl(): string {
  return process.env.NEXT_PUBLIC_GATEWAY_URL ?? "http://localhost:8083";
}

export function subscribeToMatches(
  matchId: string | undefined,
  onSnapshot: (data: MatchBody | ListSnapshot) => void,
  onUpdate: (event: EventBody) => void,
  onConnectionChange?: (connected: boolean) => void,
  EventSourceImpl: typeof EventSource = EventSource,
): () => void {
  const query = matchId ? `?match_id=${encodeURIComponent(matchId)}` : "";
  const source = new EventSourceImpl(`${baseUrl()}/events${query}`);

  source.addEventListener("snapshot", (evt) => {
    onSnapshot(JSON.parse((evt as MessageEvent).data));
  });
  source.addEventListener("update", (evt) => {
    onUpdate(JSON.parse((evt as MessageEvent).data));
  });
  source.addEventListener("open", () => onConnectionChange?.(true));
  source.addEventListener("error", () => onConnectionChange?.(false));

  return () => source.close();
}
