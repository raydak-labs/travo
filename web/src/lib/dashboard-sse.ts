import type { GetDashboardEventsData } from './api/gen';
import type { Client } from './api/gen/client';
import { createSseClient } from './api/gen/core/serverSentEvents.gen';
import { APIRequestError } from './api/request';

const EVENTS_URL = '/api/v1/dashboard/events' satisfies GetDashboardEventsData['url'];

/** Normalize all SSE line endings before the generated parser sees a chunk. */
export function normalizeEventBody(body: ReadableStream<Uint8Array>): ReadableStream<Uint8Array> {
  let pendingCR = false;
  return body.pipeThrough(
    new TransformStream<Uint8Array, Uint8Array>({
      transform(chunk, controller) {
        const out = new Uint8Array(chunk.length + 1);
        let length = 0;
        for (const byte of chunk) {
          if (pendingCR) {
            out[length++] = 10;
            pendingCR = false;
            if (byte === 10) continue;
          }
          if (byte === 13) pendingCR = true;
          else out[length++] = byte;
        }
        controller.enqueue(out.subarray(0, length));
      },
      flush(controller) {
        if (pendingCR) controller.enqueue(new Uint8Array([10]));
      },
    }),
  );
}

export type EventStreamOptions = {
  client: Client;
  signal: AbortSignal;
  /** The response proved to be a live event stream. */
  onOpen: () => void;
  onEvent: (name: string, data: unknown) => void;
};

/**
 * Reads GET /dashboard/events once until it ends or fails. Reconnecting is the caller's
 * job (fresh ownership checks, no resume ID). Resolves `unauthorized` on 401.
 */
export async function readEventStream(
  options: EventStreamOptions,
): Promise<'retry' | 'unauthorized'> {
  const { client, signal } = options;
  const transport = client.getConfig().fetch ?? globalThis.fetch;
  let failure: unknown;
  const { stream } = createSseClient({
    url: client.buildUrl({ url: EVENTS_URL }),
    credentials: 'same-origin',
    signal,
    sseMaxRetryAttempts: 1,
    fetch: async (request) => {
      signal.throwIfAborted();
      const response = await transport(request);
      if (signal.aborted) {
        await response.body?.cancel();
        signal.throwIfAborted();
      }
      checkEventResponse(response);
      options.onOpen();
      // checkEventResponse proved the body exists.
      return new Response(
        normalizeEventBody(response.body as ReadableStream<Uint8Array>),
        response,
      );
    },
    onSseError: (error) => {
      failure = error;
    },
    onSseEvent: (event) => options.onEvent(event.event ?? 'message', event.data),
  });
  for await (const event of stream) void event;
  return failure instanceof APIRequestError && failure.status === 401 ? 'unauthorized' : 'retry';
}

/** Throws unless the response is a 200 text/event-stream with a body. */
export function checkEventResponse(response: Response): void {
  if (response.status === 401) throw new APIRequestError('Session expired', 401);
  if (response.status !== 200) throw new Error(`Events HTTP ${response.status}`);
  const contentType = response.headers.get('Content-Type')?.split(';', 1)[0]?.trim().toLowerCase();
  if (contentType !== 'text/event-stream') {
    throw new Error(`Events content type: ${contentType ?? 'missing'}`);
  }
  if (!response.body) throw new Error('Events response has no body');
}
