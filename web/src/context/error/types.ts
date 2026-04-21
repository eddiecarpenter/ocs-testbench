export interface ErrorDetails {
  message: string;
  name?: string;
  stack?: string;
  cause?: unknown;
  /** ISO timestamp captured when the error was first observed. */
  time: string;
}
