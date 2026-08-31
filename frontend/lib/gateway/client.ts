import type { EventBody, MatchBody } from "./types";

export class GatewayError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "GatewayError";
    this.status = status;
  }
}

function baseUrl(): string {
  return process.env.NEXT_PUBLIC_GATEWAY_URL ?? "http://localhost:8083";
}

async function gatewayFetch<T>(path: string): Promise<T> {
  const res = await fetch(`${baseUrl()}${path}`);
  if (!res.ok) {
    throw new GatewayError(
      res.status,
      `Gateway request to ${path} failed with status ${res.status}`,
    );
  }
  return res.json() as Promise<T>;
}

export async function listMatches(status?: string): Promise<MatchBody[]> {
  const query = status ? `?status=${encodeURIComponent(status)}` : "";
  const data = await gatewayFetch<{ matches: MatchBody[] }>(`/matches${query}`);
  return data.matches;
}

export async function getMatch(matchId: string): Promise<MatchBody> {
  return gatewayFetch<MatchBody>(`/matches/${encodeURIComponent(matchId)}`);
}

export async function listMatchEvents(matchId: string): Promise<EventBody[]> {
  const data = await gatewayFetch<{ events: EventBody[] }>(
    `/matches/${encodeURIComponent(matchId)}/events`,
  );
  return data.events;
}
