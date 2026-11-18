import { afterEach, describe, expect, it, vi } from 'vitest';
import { diagnosisStatus, runDiagnostic } from './diagnostics';
import { runNetworkDiagnostic } from './gen';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({ runNetworkDiagnostic: vi.fn() }));

afterEach(() => vi.resetAllMocks());

const answer = (data: unknown, error?: unknown) =>
  ({
    data,
    error,
    request: new Request('http://localhost'),
    response: new Response(null, { status: data ? 200 : 409 }),
  }) as never;

describe('diagnostics', () => {
  it('labels success, timeout and failure', () => {
    expect(diagnosisStatus({ ok: true, timed_out: false }).kind).toBe('ok');
    expect(diagnosisStatus({ ok: false, timed_out: true }).label).toBe('Timed out');
    expect(diagnosisStatus({ ok: false, timed_out: false })).toEqual({
      kind: 'warn',
      label: 'Failed',
    });
  });

  it('sends the trimmed target and maps error codes', async () => {
    const run = vi.mocked(runNetworkDiagnostic);
    const diagnosis = {
      tool: 'ping',
      target: '10.0.1.1',
      ok: true,
      timed_out: false,
      truncated: false,
      output: 'x',
    };
    run.mockResolvedValueOnce(answer(diagnosis));
    await expect(runDiagnostic('ping', ' 10.0.1.1 ')).resolves.toEqual({ diagnosis });
    expect(run.mock.calls[0]?.[0]?.body).toEqual({ tool: 'ping', target: '10.0.1.1' });
    run.mockResolvedValueOnce(answer(undefined, { code: 'diagnostics_busy' }));
    await expect(runDiagnostic('ping', 'x')).resolves.toEqual({
      error: 'Another diagnostic is running; try again when it finishes.',
    });
    run.mockRejectedValueOnce(new Error('offline'));
    await expect(runDiagnostic('lookup', 'x')).resolves.toEqual({
      error: 'The diagnostic could not run.',
    });
  });
});
