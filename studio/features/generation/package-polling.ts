import type { GeneratorID } from '../../lib/api/contracts.ts';
import type { deliveryAPI, Generation } from '../../lib/api/delivery.ts';
import { nextPollDelay } from './generation-state.ts';
import { packageContractError } from './package-contract.ts';

export function abortableDelay(milliseconds: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal.aborted) {
      reject(signal.reason);
      return;
    }
    const abort = () => {
      clearTimeout(timer);
      reject(signal.reason);
    };
    const timer = setTimeout(() => {
      signal.removeEventListener('abort', abort);
      resolve();
    }, milliseconds);
    signal.addEventListener('abort', abort, { once: true });
  });
}

// Cancellation is checked after every await, including clients that ignore AbortSignal.
export async function resolveGeneratedPackage(
  api: Pick<typeof deliveryAPI, 'getGeneration' | 'getPackage'>,
  project: string,
  initial: Generation,
  signal: AbortSignal,
  expectedGenerator: GeneratorID,
  onProgress: (generation: Generation) => void,
  delay = abortableDelay,
) {
  signal.throwIfAborted();
  let current = initial;
  let attempt = 1;
  while (current.status === 'queued' || current.status === 'running') {
    await delay(nextPollDelay(attempt), signal);
    signal.throwIfAborted();
    current = await api.getGeneration(project, current.id, signal);
    signal.throwIfAborted();
    onProgress(current);
    attempt += 1;
  }
  if (current.status === 'failed') {
    throw new Error(current.error?.message || '패키지 생성에 실패했습니다.');
  }
  if (current.status !== 'succeeded' || !current.packageId) {
    throw new Error('생성 결과에 package ID가 없습니다.');
  }
  const result = await api.getPackage(project, current.packageId, signal);
  signal.throwIfAborted();
  const error = packageContractError(result, expectedGenerator);
  if (error) throw new Error(error);
  return result;
}
