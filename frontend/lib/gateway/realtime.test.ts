import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { subscribeToMatches } from "./realtime";

type Listener = (event: MessageEvent) => void;

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  url: string;
  closed = false;
  private listeners = new Map<string, Listener[]>();

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: Listener) {
    const existing = this.listeners.get(type) ?? [];
    existing.push(listener);
    this.listeners.set(type, existing);
  }

  close() {
    this.closed = true;
  }

  emit(type: string, data?: unknown) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener({ data: JSON.stringify(data) } as MessageEvent);
    }
  }
}

beforeEach(() => {
  FakeEventSource.instances = [];
  vi.stubEnv("NEXT_PUBLIC_GATEWAY_URL", "http://gateway.test");
});

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("subscribeToMatches", () => {
  it("opens /events with no query when matchId is omitted", () => {
    subscribeToMatches(
      undefined,
      vi.fn(),
      vi.fn(),
      undefined,
      FakeEventSource as unknown as typeof EventSource,
    );
    expect(FakeEventSource.instances[0].url).toBe("http://gateway.test/events");
  });

  it("opens /events?match_id=<id> when matchId is given", () => {
    subscribeToMatches(
      "m1",
      vi.fn(),
      vi.fn(),
      undefined,
      FakeEventSource as unknown as typeof EventSource,
    );
    expect(FakeEventSource.instances[0].url).toBe(
      "http://gateway.test/events?match_id=m1",
    );
  });

  it("routes a snapshot frame to onSnapshot", () => {
    const onSnapshot = vi.fn();
    subscribeToMatches(
      "m1",
      onSnapshot,
      vi.fn(),
      undefined,
      FakeEventSource as unknown as typeof EventSource,
    );
    const source = FakeEventSource.instances[0];
    const match = {
      match_id: "m1",
      sport: "football",
      status: "live",
      home_score: 0,
      away_score: 0,
      clock_mins: 0,
    };
    source.emit("snapshot", match);
    expect(onSnapshot).toHaveBeenCalledWith(match);
  });

  it("routes an update frame to onUpdate, not onSnapshot", () => {
    const onSnapshot = vi.fn();
    const onUpdate = vi.fn();
    subscribeToMatches(
      "m1",
      onSnapshot,
      onUpdate,
      undefined,
      FakeEventSource as unknown as typeof EventSource,
    );
    const source = FakeEventSource.instances[0];
    const event = { type: "goal", sequence: 1, payload: { team: "home" } };
    source.emit("update", event);
    expect(onUpdate).toHaveBeenCalledWith(event);
    expect(onSnapshot).not.toHaveBeenCalled();
  });

  it("calls onConnectionChange(true) on open and (false) on error", () => {
    const onConnectionChange = vi.fn();
    subscribeToMatches(
      undefined,
      vi.fn(),
      vi.fn(),
      onConnectionChange,
      FakeEventSource as unknown as typeof EventSource,
    );
    const source = FakeEventSource.instances[0];
    source.emit("open");
    expect(onConnectionChange).toHaveBeenCalledWith(true);
    source.emit("error");
    expect(onConnectionChange).toHaveBeenCalledWith(false);
  });

  it("closes the EventSource when the returned cleanup function is called", () => {
    const cleanup = subscribeToMatches(
      undefined,
      vi.fn(),
      vi.fn(),
      undefined,
      FakeEventSource as unknown as typeof EventSource,
    );
    const source = FakeEventSource.instances[0];
    expect(source.closed).toBe(false);
    cleanup();
    expect(source.closed).toBe(true);
  });
});
