export type IdempotencyAttempt = { signature: string; key: string };

export function idempotencyAttempt(
  previous: IdempotencyAttempt | null,
  signature: string,
  createKey: () => string,
): IdempotencyAttempt {
  return previous?.signature === signature
    ? previous
    : { signature, key: createKey() };
}

export function nextPollDelay(attempt: number) {
  return Math.min(1000 * Math.max(attempt, 1), 5000);
}
