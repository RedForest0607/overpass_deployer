'use client';

import { useState } from 'react';
import { Textarea } from '@/components/ui/textarea';
import type { JavaDeployment, VMDeployment } from '../../lib/api/contracts';

type Deployment = JavaDeployment | VMDeployment;

export default function DeploymentEditor({
  kind,
  value,
  onChange,
}: {
  kind: 'java' | 'vm';
  value?: Deployment;
  onChange: (value: Deployment) => void;
}) {
  const [draft, setDraft] = useState(() => format(value ?? defaults(kind)));
  const [error, setError] = useState('');

  function apply() {
    try {
      const parsed: unknown = JSON.parse(draft);
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
        throw new Error('JSON 객체를 입력하세요.');
      }
      onChange(parsed as Deployment);
      setDraft(format(parsed));
      setError('');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'JSON을 확인하세요.');
    }
  }

  return (
    <div className="field">
      <span>Crosswalk 배포 상세 (JSON)</span>
      <Textarea
        value={draft}
        rows={12}
        spellCheck={false}
        aria-invalid={!!error}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={apply}
      />
      {error ? (
        <small style={{ color: '#b34e4e' }}>{error}</small>
      ) : (
        <small>bootstrap, 파일, utility, JVM·script 옵션을 설정합니다.</small>
      )}
    </div>
  );
}

function defaults(kind: 'java' | 'vm'): Deployment {
  if (kind === 'vm') {
    return {
      bootstrap: { packages: [], timezone: 'Asia/Seoul' },
      files: [],
      utilities: [],
    };
  }
  return {
    minHeap: '256m',
    javaOptions: [],
    extraOptions: [],
    configFiles: [],
    extraFiles: [],
  };
}

function format(value: unknown) {
  return JSON.stringify(value, null, 2);
}
