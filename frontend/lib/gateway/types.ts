export type MatchStatus = "scheduled" | "live" | "half_time" | "full_time";

export interface MatchBody {
  match_id: string;
  sport: string;
  status: MatchStatus;
  home_team: string;
  away_team: string;
  home_score: number;
  away_score: number;
  clock_mins: number;
}

export interface EventBody {
  type: string;
  sequence: number;
  payload: Record<string, unknown>;
  match_id?: string;
}
