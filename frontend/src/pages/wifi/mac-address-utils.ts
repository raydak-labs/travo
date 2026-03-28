/** Locally generated random MAC (locally administered unicast). */
export function generateRandomMac(): string {
  const hex = () =>
    Math.floor(Math.random() * 256)
      .toString(16)
      .padStart(2, '0');
  const first = (Math.floor(Math.random() * 256) & 0xfe) | 0x02;
  return [first.toString(16).padStart(2, '0'), hex(), hex(), hex(), hex(), hex()].join(':');
}
