/** What a confirmation dialog asks; the page decides the words, the dialog the layout. */
export type ConfirmRequest = {
  title: string;
  description: string;
  /** Label of the button that goes ahead, e.g. "Restart router". */
  action: string;
  /** `danger` marks a change that can cut the user off or lose data. */
  tone?: 'danger' | 'default';
};

/**
 * One confirmation at a time per owner. `ask` resolves true only when the user picks the
 * action; Cancel, Escape, an outside click or a newer `ask` resolve false.
 */
export function createConfirm() {
  let request = $state<ConfirmRequest>();
  let resolve: ((ok: boolean) => void) | undefined;

  function answer(ok: boolean) {
    const done = resolve;
    resolve = undefined;
    request = undefined;
    done?.(ok);
  }

  return {
    get request() {
      return request;
    },
    ask(next: ConfirmRequest): Promise<boolean> {
      answer(false);
      request = next;
      return new Promise((done) => {
        resolve = done;
      });
    },
    answer,
  };
}

export type Confirm = ReturnType<typeof createConfirm>;
