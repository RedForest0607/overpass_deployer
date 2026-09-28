'use client';

import { useEffect, useState } from 'react';
import { deliveryAPI } from '../../lib/api/delivery';

type Preview = {
  key: string;
  content: string;
  truncated: boolean;
  error: string;
};

export function usePackagePreview(
  projectId?: string,
  packageId?: string,
  fileId?: string,
) {
  const key = JSON.stringify([projectId, packageId, fileId]);
  const [preview, setPreview] = useState<Preview | null>(null);
  useEffect(() => {
    if (!projectId || !packageId || !fileId) return;
    const controller = new AbortController();
    void deliveryAPI
      .previewFile(projectId, packageId, fileId, controller.signal)
      .then((value) => {
        if (!controller.signal.aborted)
          setPreview({
            key,
            content: value.content,
            truncated: value.truncated,
            error: '',
          });
      })
      .catch((cause) => {
        if (!controller.signal.aborted)
          setPreview({
            key,
            content: '',
            truncated: false,
            error:
              cause instanceof Error
                ? cause.message
                : '파일 미리보기에 실패했습니다.',
          });
      });
    return () => controller.abort();
  }, [projectId, packageId, fileId, key]);
  return preview?.key === key
    ? preview
    : { content: '', truncated: false, error: '' };
}
