/** One snapshot in the Pyramidize history (the trace log). */
export interface TraceEntry {
  id: string;
  label: string;
  snapshot: string;
  timestamp: Date;
  /**
   * The result the snapshot is of; reverting writes it back there (and shows
   * that result). None for a snapshot of the original text.
   */
  resultId?: string;
}

/** The time a snapshot was taken, as the history list shows it. */
export function formatTraceTime(d: Date): string {
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}
