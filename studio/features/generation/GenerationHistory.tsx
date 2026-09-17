'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { History, RefreshCw, RotateCcw } from 'lucide-react';
import type { Environment } from '../../lib/api/contracts';
import { deliveryAPI, type Generation } from '../../lib/api/delivery';

const statusLabel: Record<Generation['status'], string> = {
  queued: '대기 중',
  running: '생성 중',
  succeeded: '완료',
  failed: '실패',
};

const formatDate = (value: string) =>
  new Intl.DateTimeFormat('ko-KR', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));

export default function GenerationHistory({
  projectId,
  environment,
  activeGeneration,
  onResume,
}: {
  projectId: string;
  environment: Environment;
  activeGeneration: Generation | null;
  onResume: (generation: Generation) => Promise<void>;
}) {
  const [items, setItems] = useState<Generation[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [actionId, setActionId] = useState('');
  const [message, setMessage] = useState('');
  const retryKeys = useRef(new Map<string, string>());
  const loadVersion = useRef(0);
  const visibleItems = useMemo(() => {
    if (!activeGeneration || activeGeneration.environment !== environment) {
      return items;
    }
    return [
      activeGeneration,
      ...items.filter((item) => item.id !== activeGeneration.id),
    ].slice(0, 10);
  }, [activeGeneration, environment, items]);

  async function load(nextCursor?: string, append = false) {
    const version = ++loadVersion.current;
    setLoading(true);
    setMessage('');
    try {
      const page = await deliveryAPI.listGenerations(
        projectId,
        environment,
        nextCursor,
      );
      if (version !== loadVersion.current) return;
      setItems((current) => (append ? [...current, ...page.items] : page.items));
      setCursor(page.nextCursor);
    } catch (cause) {
      if (version !== loadVersion.current) return;
      setMessage(
        cause instanceof Error ? cause.message : '생성 이력을 불러오지 못했습니다.',
      );
    } finally {
      if (version === loadVersion.current) setLoading(false);
    }
  }

  useEffect(() => {
    const version = ++loadVersion.current;
    void deliveryAPI
      .listGenerations(projectId, environment)
      .then((page) => {
        if (version !== loadVersion.current) return;
        setItems(page.items);
        setCursor(page.nextCursor);
      })
      .catch((cause) => {
        if (version !== loadVersion.current) return;
        setMessage(
          cause instanceof Error
            ? cause.message
            : '생성 이력을 불러오지 못했습니다.',
        );
      })
      .finally(() => {
        if (version === loadVersion.current) setLoading(false);
      });
    return () => {
      loadVersion.current += 1;
    };
  }, [projectId, environment]);

  async function retry(generation: Generation) {
    setActionId(generation.id);
    setMessage('');
    try {
      let key = retryKeys.current.get(generation.id);
      if (!key) {
        key = crypto.randomUUID();
        retryKeys.current.set(generation.id, key);
      }
      const queued = await deliveryAPI.retryGeneration(
        projectId,
        generation.id,
        key,
      );
      setItems((current) => [queued, ...current]);
      await onResume(queued);
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : '생성 작업을 다시 시도하지 못했습니다.',
      );
    } finally {
      setActionId('');
    }
  }

  async function resume(generation: Generation) {
    setActionId(generation.id);
    setMessage('');
    try {
      await onResume(generation);
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : '생성 작업을 다시 열지 못했습니다.',
      );
    } finally {
      setActionId('');
    }
  }

  return (
    <section className="generation-history">
      <div className="generation-history-heading">
        <div>
          <History size={17} />
          <strong>생성 이력</strong>
          <span>{environment.toUpperCase()} · 최신순</span>
        </div>
        <button disabled={loading || !!actionId} onClick={() => void load()}>
          <RefreshCw size={14} /> 새로고침
        </button>
      </div>
      {visibleItems.length ? (
        <div className="generation-list">
          {visibleItems.map((item) => (
            <article key={item.id}>
              <div>
                <span className={'generation-badge ' + item.status}>
                  {statusLabel[item.status]}
                </span>
                <strong>revision {item.revision}</strong>
                {item.retryOfGenerationId && <small>재시도 작업</small>}
              </div>
              <p>
                {formatDate(item.createdAt)} · {item.attempts}차 시도
                {item.error ? ` · ${item.error.message}` : ''}
              </p>
              {item.status === 'failed' ? (
                <button
                  disabled={loading || !!actionId}
                  onClick={() => void retry(item)}
                >
                  <RotateCcw size={14} /> snapshot 재시도
                </button>
              ) : (
                <button
                  disabled={loading || !!actionId}
                  onClick={() => void resume(item)}
                >
                  {item.status === 'succeeded' ? '패키지 열기' : '작업 다시 연결'}
                </button>
              )}
            </article>
          ))}
        </div>
      ) : (
        <div className="generation-history-empty">
          {loading ? '생성 이력을 불러오는 중…' : '이 환경의 생성 이력이 없습니다.'}
        </div>
      )}
      {cursor && (
        <button
          className="generation-more"
          disabled={loading || !!actionId}
          onClick={() => void load(cursor, true)}
        >
          이전 이력 더 보기
        </button>
      )}
      {message && <output className="generation-history-message">{message}</output>}
    </section>
  );
}
