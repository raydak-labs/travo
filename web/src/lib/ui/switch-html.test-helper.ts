/** Reads a server-rendered bits-ui Switch by id: whether it is on and whether it is disabled. */
export function switchState(
  body: string,
  id: string,
): { checked: boolean; disabled: boolean } | null {
  const tag = body.match(new RegExp(`<button[^>]*\\bid="${id}"[^>]*>`))?.[0];
  if (!tag) return null;
  return {
    checked: /aria-checked="true"/.test(tag),
    // Attribute only: the class list also mentions `data-disabled:` variants.
    disabled: /\sdisabled(="")?(?=[\s/>])/.test(tag.replace(/class="[^"]*"/, '')),
  };
}
