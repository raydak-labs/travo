import { describe, expect, it } from 'vitest';
import { createSseClient, type StreamEvent } from './api/gen/core/serverSentEvents.gen';
import { normalizeEventBody } from './dashboard-sse';
import { deferred } from './deferred.test-utils';

function stream(chunks: Uint8Array[]): ReadableStream<Uint8Array> {
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(chunk);
      controller.close();
    },
  });
}

async function consume(
  body: ReadableStream<Uint8Array>,
  events: StreamEvent[],
  signal = new AbortController().signal,
  requests: Request[] = [],
): Promise<void> {
  const { stream } = createSseClient({
    url: 'http://daemon.test/api/v1/dashboard/events',
    signal,
    sseMaxRetryAttempts: 1,
    fetch: async (request) => {
      requests.push(request as Request);
      return new Response(normalizeEventBody(body), {
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
      });
    },
    onSseEvent: (event) => {
      if (event.event) events.push(event);
    },
  });
  for await (const event of stream) void event;
}

const encode = (text: string) => new TextEncoder().encode(text);

describe('dashboard SSE framing', () => {
  it('handles comments, named events, multi-line data and CRLF split between chunks', async () => {
    const events: StreamEvent[] = [];
    await consume(
      stream([
        encode(': connected\r'),
        encode('\n\nevent: resources\r\ndata: {"load":[0.1,\r'),
        encode('\ndata: 0.2,0.3]}\r\n\r\nevent: throughput\ndata: {"interfaces":[]}\n\n'),
      ]),
      events,
    );
    expect(events.map(({ event, data }) => ({ event, data }))).toEqual([
      { event: 'resources', data: { load: [0.1, 0.2, 0.3] } },
      { event: 'throughput', data: { interfaces: [] } },
    ]);
  });

  it('does not resume an event ID on a replacement subscription', async () => {
    const requests: Request[] = [];
    await consume(
      stream([encode('id: 42\nretry: 9000\nevent: connectivity\ndata: {"internet":"online"}\n\n')]),
      [],
      undefined,
      requests,
    );
    await consume(stream([encode(': connected\n\n')]), [], undefined, requests);
    expect(requests[1]?.headers.get('Last-Event-ID')).toBeNull();
  });

  it('discards an incomplete event at EOF', async () => {
    const events: StreamEvent[] = [];
    await consume(stream([encode('event: connectivity\ndata: {"internet":"online"}')]), events);
    expect(events).toEqual([]);
  });

  it('decodes a UTF-8 code point split across chunks', async () => {
    const encoded = encode(
      ': connected\n\nevent: connectivity\ndata: {"active_interface":"é","internet":"online"}\n\n',
    );
    const split = encoded.indexOf(0xc3);
    const events: StreamEvent[] = [];
    await consume(stream([encoded.slice(0, split + 1), encoded.slice(split + 1)]), events);
    expect(events.map(({ event, data }) => ({ event, data }))).toEqual([
      {
        event: 'connectivity',
        data: { active_interface: 'é', internet: 'online' },
      },
    ]);
  });

  it('cancels a pending read when ownership is aborted', async () => {
    const controller = new AbortController();
    const cancelled = deferred<void>();
    const body = new ReadableStream<Uint8Array>({
      pull() {
        return new Promise(() => {});
      },
      cancel() {
        cancelled.resolve();
      },
    });
    const consumed = consume(body, [], controller.signal);
    await Promise.resolve();
    await Promise.resolve();
    controller.abort();
    await consumed;
    await cancelled.promise;
  });
});
