export class APIError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

/** Same-origin only. No automatic retry of writes and no silent sample fallback. */
export async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (
    options.body &&
    !(options.body instanceof FormData) &&
    !headers.has('Content-Type')
  ) {
    headers.set('Content-Type', 'application/json');
  }
  const response = await fetch('/api/v1' + path, {
    ...options,
    headers,
  });
  const body = (await response.json().catch(() => null)) as {
    error?: { code?: string; message?: string };
  } | null;
  if (!response.ok) {
    throw new APIError(
      response.status,
      body?.error?.code ?? 'HTTP_ERROR',
      body?.error?.message ?? 'API 요청에 실패했습니다.',
    );
  }
  if (body === null)
    throw new APIError(
      response.status,
      'INVALID_RESPONSE',
      '서버 JSON 응답을 확인하세요.',
    );
  return body as T;
}

export async function requestBlob(
  path: string,
  options: RequestInit = {},
): Promise<{ blob: Blob; filename?: string }> {
  const response = await fetch('/api/v1' + path, options);
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as {
      error?: { code?: string; message?: string };
    } | null;
    throw new APIError(
      response.status,
      body?.error?.code ?? 'HTTP_ERROR',
      body?.error?.message ?? '파일 요청에 실패했습니다.',
    );
  }
  const disposition = response.headers.get('Content-Disposition') ?? '';
  const filename = disposition.match(/filename="([^"]+)"/)?.[1];
  return { blob: await response.blob(), filename };
}
