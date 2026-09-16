'use client';
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  deliveryAPI,
  type Artifact,
  type ArtifactKind,
} from '../../lib/api/delivery';

export function useArtifacts(projectId?: string) {
  const [items, setItems] = useState<Artifact[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const currentProject = useRef(projectId);

  const refresh = useCallback(
    async (signal?: AbortSignal) => {
      if (!projectId) {
        setItems([]);
        return;
      }
      setBusy(true);
      setError('');
      try {
        setItems(await deliveryAPI.listArtifacts(projectId, signal));
      } catch (cause) {
        if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
          setError(
            cause instanceof Error
              ? cause.message
              : '파일 목록을 불러오지 못했습니다.',
          );
        }
      } finally {
        if (!signal?.aborted) setBusy(false);
      }
    },
    [projectId],
  );

  useEffect(() => {
    currentProject.current = projectId;
    const controller = new AbortController();
    void Promise.resolve().then(() => {
      if (!controller.signal.aborted) return refresh(controller.signal);
    });
    return () => controller.abort();
  }, [projectId, refresh]);

  async function upload(kind: ArtifactKind, file: File, displayName?: string) {
    if (!projectId || busy) return null;
    setBusy(true);
    setError('');
    try {
      const created = await deliveryAPI.uploadArtifact(
        projectId,
        kind,
        file,
        displayName,
      );
      if (currentProject.current !== projectId) return null;
      setItems((current) => [created, ...current]);
      return created;
    } catch (cause) {
      if (currentProject.current === projectId)
        setError(
          cause instanceof Error
            ? cause.message
            : '파일을 업로드하지 못했습니다.',
        );
      return null;
    } finally {
      if (currentProject.current === projectId) setBusy(false);
    }
  }

  function readText(artifactId: string, signal?: AbortSignal) {
    if (!projectId)
      return Promise.reject(new Error('프로젝트를 먼저 선택하세요.'));
    return deliveryAPI.getArtifactContent(projectId, artifactId, signal);
  }

  return {
    items,
    busy,
    error,
    upload,
    readText,
    refresh: () => void refresh(),
  };
}
