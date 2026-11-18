export type CopyDeps = {
  clipboard?: Pick<Clipboard, 'writeText'>;
  /** Selects the visible text and runs the legacy copy command; false when the browser refused. */
  selectAndCopy: () => boolean;
};

/**
 * Copies text. The admin UI is usually plain http on the LAN, which is not a secure context, so
 * `navigator.clipboard` is often missing: fall back to selecting the field and the copy command.
 */
export async function copyText(text: string, deps: CopyDeps): Promise<boolean> {
  if (deps.clipboard) {
    try {
      await deps.clipboard.writeText(text);
      return true;
    } catch {
      // Permission denied or not focused: try the selection path below.
    }
  }
  try {
    return deps.selectAndCopy();
  } catch {
    return false;
  }
}
