import type { SseEvent } from './events';
import { SSE_EVENT_TYPES } from './events';
import type { EventStreamLike } from './transport';
import { openEventStream } from './transport';

export type SseStatus = 'idle' | 'connecting' | 'open' | 'closed';

type EventHandler = (event: SseEvent) => void;
type StatusHandler = (status: SseStatus) => void;

interface SseClientOptions {
  url: string;
  /** Called for every decoded event, regardless of type. */
  onEvent: EventHandler;
  /** Called when the connection status changes. Optional. */
  onStatus?: StatusHandler;
  /**
   * Called once the underlying `EventSource` reports `error` at a moment
   * when the browser will auto-reconnect. Use it to invalidate caches so
   * the next paint is re-fetched truth.
   */
  onReconnectHint?: () => void;
}

/**
 * Thin wrapper around an `EventSource`-like stream.
 *
 * - Multiplexes typed events onto a single `onEvent` callback
 * - Exposes a status lifecycle (`idle → connecting → open → closed`)
 * - Delegates reconnection to the underlying transport (browsers do this
 *   natively for `EventSource`); on each transient error we emit a
 *   reconnect hint so callers can invalidate their caches.
 *
 * The client owns one stream at a time. Call `close()` before re-opening.
 */
export class SseClient {
  private readonly opts: SseClientOptions;
  private stream: EventStreamLike | undefined;
  private status: SseStatus = 'idle';
  private readonly listeners = new Map<
    string,
    (ev: MessageEvent) => void
  >();
  private openListener: (() => void) | undefined;
  private errorListener: (() => void) | undefined;

  constructor(opts: SseClientOptions) {
    this.opts = opts;
  }

  open(): void {
    if (this.stream) return;
    this.setStatus('connecting');
    const stream = openEventStream(this.opts.url);
    this.stream = stream;

    // Native EventSource fires a generic "open"/"error" on the instance;
    // our minimal interface doesn't model them, so we cast.
    const raw = stream as unknown as {
      addEventListener: (
        type: string,
        handler: (ev: Event | MessageEvent) => void,
      ) => void;
    };

    this.openListener = () => this.setStatus('open');
    this.errorListener = () => {
      // `EventSource` auto-reconnects on transient errors. If readyState
      // drops to CLOSED (2) the connection is terminal.
      if (stream.readyState === 2) {
        this.setStatus('closed');
      } else {
        this.setStatus('connecting');
        this.opts.onReconnectHint?.();
      }
    };

    raw.addEventListener('open', this.openListener);
    raw.addEventListener('error', this.errorListener);

    for (const type of SSE_EVENT_TYPES) {
      const listener = (ev: MessageEvent) => {
        try {
          const data = JSON.parse(ev.data);
          this.opts.onEvent({ type, data } as SseEvent);
        } catch (err) {
          // A malformed event should never kill the stream — log and drop.
          console.warn(`[sse] dropped malformed ${type} event`, err);
        }
      };
      this.listeners.set(type, listener);
      stream.addEventListener(type, listener);
    }
  }

  close(): void {
    const stream = this.stream;
    if (!stream) return;

    for (const [type, listener] of this.listeners) {
      stream.removeEventListener(type, listener);
    }
    this.listeners.clear();

    const raw = stream as unknown as {
      removeEventListener: (type: string, handler: () => void) => void;
    };
    if (this.openListener) raw.removeEventListener('open', this.openListener);
    if (this.errorListener) raw.removeEventListener('error', this.errorListener);
    this.openListener = undefined;
    this.errorListener = undefined;

    stream.close();
    this.stream = undefined;
    this.setStatus('closed');
  }

  getStatus(): SseStatus {
    return this.status;
  }

  private setStatus(next: SseStatus): void {
    if (this.status === next) return;
    this.status = next;
    this.opts.onStatus?.(next);
  }
}
