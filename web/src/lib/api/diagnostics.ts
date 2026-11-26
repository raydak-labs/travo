import { m } from '../paraglide/messages.js';
import type { StatusKind } from '../ui/status';
import { client } from './client';
import { type Diagnosis, runNetworkDiagnostic } from './gen';
import type { Client } from './gen/client';

export type { Diagnosis };

export const DIAGNOSTIC_TOOLS = ['ping', 'traceroute', 'lookup'] as const;
export type DiagnosticTool = (typeof DIAGNOSTIC_TOOLS)[number];

export type DiagnosticOutcome = { diagnosis: Diagnosis } | { error: string };

/** Maps the API's stable error codes to a message; never parses error text. */
export function diagnosticError(code: string | undefined): string {
  switch (code) {
    case 'invalid_target':
      return m.diag_err_target();
    case 'diagnostics_busy':
      return m.diag_err_busy();
    case 'rate_limited':
      return m.diag_err_rate();
    default:
      return m.diag_err_failed();
  }
}

/** Badge for a finished run: a timeout is called out, any other failure is a warning. */
export function diagnosisStatus(d: Pick<Diagnosis, 'ok' | 'timed_out'>): {
  kind: StatusKind;
  label: string;
} {
  if (d.ok) return { kind: 'ok', label: m.diag_ok() };
  return { kind: 'warn', label: d.timed_out ? m.diag_timed_out() : m.diag_failed() };
}

export async function runDiagnostic(
  tool: DiagnosticTool,
  target: string,
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<DiagnosticOutcome> {
  try {
    const { data, error } = await runNetworkDiagnostic({
      body: { tool, target: target.trim() },
      signal,
      client: requestClient,
    });
    if (data) return { diagnosis: data };
    return { error: diagnosticError((error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: diagnosticError(undefined) };
  }
}
