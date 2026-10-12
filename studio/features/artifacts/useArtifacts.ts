'use client';
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  deliveryAPI,
  type Artifact,
  type ArtifactKind,
} from '../../lib/api/delivery';
import { ArtifactRequests } from './artifact-requests';

type ArtifactState = {
  projectId?: string;
  items: Artifact[];
  busy: boolean;
  error: string;
};

export function useArtifacts(projectId?: string) {
  const [state, setState] = useState<ArtifactState>({
    items: [],
    busy: false,
    error: '',
  });
  const requests = useRef(new ArtifactRequests());

  const refresh = useCallback(async () => {
    if (!projectId) return;
    const request = requests.current.begin(projectId, 'list');
    if (!request) return;
    setState((current) => ({
      projectId,
      items: current.projectId === projectId ? current.items : [],
      busy: true,
      error: '',
    }));
    try {
      const items = await deliveryAPI.listArtifacts(
        projectId,
        request.controller.signal,
      );
      if (requests.current.isCurrent(request))
        setState({ projectId, items, busy: false, error: '' });
    } catch (cause) {
      if (requests.current.isCurrent(request))
        setState((current) => ({
          ...current,
          busy: false,
          error:
            cause instanceof Error
              ? cause.message
              : '파일 목록을 불러오지 못했습니다.',
        }));
    } finally {
      requests.current.finish(request);
    }
  }, [projectId]);

  useEffect(() => {
    const scope = requests.current;
    scope.cancel();
    let disposed = false;
    void Promise.resolve().then(() => {
      if (!disposed) return refresh();
    });
    return () => {
      disposed = true;
      scope.cancel();
    };
  }, [refresh]);

  async function upload(kind: ArtifactKind, file: File, displayName?: string) {
    if (!projectId) return null;
    const request = requests.current.begin(projectId, 'upload');
    if (!request) return null;
    setState((current) => ({
      projectId,
      items: current.projectId === projectId ? current.items : [],
      busy: true,
      error: '',
    }));
    try {
      const created = await deliveryAPI.uploadArtifact(
        projectId,
        kind,
        file,
        displayName,
        request.controller.signal,
      );
      if (!requests.current.isCurrent(request)) return null;
      setState((current) => ({
        ...current,
        items: [created, ...current.items],
        busy: false,
      }));
      return created;
    } catch (cause) {
      if (requests.current.isCurrent(request))
        setState((current) => ({
          ...current,
          busy: false,
          error:
            cause instanceof Error
              ? cause.message
              : '파일을 업로드하지 못했습니다.',
        }));
      return null;
    } finally {
      requests.current.finish(request);
    }
  }

  function readText(artifactId: string, signal?: AbortSignal) {
    if (!projectId)
      return Promise.reject(new Error('프로젝트를 먼저 선택하세요.'));
    return deliveryAPI.getArtifactContent(projectId, artifactId, signal);
  }

  const matches = state.projectId === projectId;
  return {
    items: matches ? state.items : [],
    busy: !!projectId && (!matches || state.busy),
    error: matches ? state.error : '',
    upload,
    readText,
    refresh: () => void refresh(),
  };
}
