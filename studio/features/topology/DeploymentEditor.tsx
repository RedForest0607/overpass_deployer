'use client';

import { useState } from 'react';
import type {
  JavaDeployment,
  UtilityDeployment,
  VMDeployment,
} from '../../lib/api/contracts';
import JsonEditorTextarea from './JsonEditorTextarea';

type Deployment = JavaDeployment | VMDeployment | UtilityDeployment;
type DeploymentKind = 'java' | 'vm' | 'redis' | 'elasticsearch';

export default function DeploymentEditor({
  kind,
  value,
  onChange,
}: {
  kind: DeploymentKind;
  value?: Deployment;
  onChange: (value: Deployment) => void;
}) {
  const [draft, setDraft] = useState(() => format(value ?? defaults(kind)));
  const [error, setError] = useState('');

  function apply(source = draft): string | void {
    try {
      const parsed: unknown = JSON.parse(source);
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
        throw new Error('JSON 객체를 입력하세요.');
      }
      onChange(parsed as Deployment);
      setDraft(format(parsed));
      setError('');
    } catch (cause) {
      const message =
        cause instanceof Error ? cause.message : 'JSON을 확인하세요.';
      setError(message);
      return message;
    }
  }

  return (
    <div className="field">
      <span>Crosswalk 배포 상세 (JSON)</span>
      <JsonEditorTextarea
        value={draft}
        rows={12}
        spellCheck={false}
        aria-invalid={!!error}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={() => apply()}
        expandedTitle="Crosswalk 배포 상세 편집"
        expandedDescription="bootstrap, 파일, utility, JVM·script 옵션을 넓은 화면에서 수정합니다."
        onExpandedApply={apply}
      />
      {error ? (
        <small style={{ color: '#b34e4e' }}>{error}</small>
      ) : (
        <small>bootstrap, 파일, utility, JVM·script 옵션을 설정합니다.</small>
      )}
    </div>
  );
}

function defaults(kind: DeploymentKind): Deployment {
  if (kind === 'vm') {
    return {
      bootstrap: { packages: [], timezone: 'Asia/Seoul' },
      files: [],
      utilities: [],
    };
  }
  if (kind === 'redis' || kind === 'elasticsearch') {
    return {
      directories: [kind === 'redis' ? '/app/redis' : '/app/elasticsearch'],
      files: [],
      services: [],
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
