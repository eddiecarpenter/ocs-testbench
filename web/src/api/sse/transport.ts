/**
 * Minimal `EventSource`-compatible interface the SSE client depends on.
 *
 * We don't use the full DOM `EventSource` type so that the mock transport
 * (used when `VITE_USE_MOCK_API` is on) can provide an in-memory emitter
 * without having to subclass the real thing.
 */
export interface EventStreamLike {
  addEventListener(type: string, listener: (ev: MessageEvent) => void): void;
  removeEventListener(type: string, listener: (ev: MessageEvent) => void): void;
  close(): void;
  /** 0 = CONNECTING, 1 = OPEN, 2 = CLOSED (matches the DOM spec). */
  readonly readyState: number;
}

export type EventStreamFactory = (url: string) => EventStreamLike;

const nativeFactory: EventStreamFactory = (url) =>
  new EventSource(url) as unknown as EventStreamLike;

let factory: EventStreamFactory = nativeFactory;

/** Open an event stream using the currently-registered factory. */
export function openEventStream(url: string): EventStreamLike {
  return factory(url);
}

/**
 * Replace the factory. Used by the mock layer to install an in-memory
 * emitter when the mock API is enabled.
 */
export function setEventStreamFactory(fn: EventStreamFactory): void {
  factory = fn;
}

/** Restore the native `EventSource` factory. Exposed for tests. */
export function resetEventStreamFactory(): void {
  factory = nativeFactory;
}
