/** HTTP failure preserved for ownership-aware callers; no response means transport failure. */
export class APIRequestError extends Error {
  constructor(
    message: string,
    readonly status?: number,
  ) {
    super(message);
  }
}
