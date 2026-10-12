'use client';

import { useState } from 'react';
import type { DeploymentProfile } from '../../lib/api/contracts';
import JsonEditorTextarea from './JsonEditorTextarea';

export default function DeploymentProfileEditor({
  value,
  onChange,
}: {
  value: DeploymentProfile;
  onChange: (value: DeploymentProfile) => void;
}) {
  return (
    <ProfileDraft
      key={JSON.stringify(value)}
      value={value}
      onChange={onChange}
    />
  );
}

function ProfileDraft({
  value,
  onChange,
}: {
  value: DeploymentProfile;
  onChange: (value: DeploymentProfile) => void;
}) {
  const [draft, setDraft] = useState(() => JSON.stringify(value, null, 2));
  const [error, setError] = useState('');

  function apply(source = draft): string | void {
    try {
      const parsed = JSON.parse(source) as Partial<DeploymentProfile>;
      if (!parsed || typeof parsed !== 'object' || !parsed.ssh) {
        throw new Error('ssh를 포함한 JSON 객체를 입력하세요.');
      }
      onChange(parsed as DeploymentProfile);
      setDraft(JSON.stringify(parsed, null, 2));
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
      <span>공통 Crosswalk 설정</span>
      <JsonEditorTextarea
        value={draft}
        rows={12}
        spellCheck={false}
        aria-invalid={!!error}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={() => apply()}
        expandedTitle="공통 Crosswalk 설정 편집"
        expandedDescription="SSH, bastion, 공통 bootstrap과 디렉터리 소유권 설정을 넓은 화면에서 수정합니다."
        onExpandedApply={apply}
      />
      <small style={error ? { color: '#b34e4e' } : undefined}>
        {error || 'SSH, bastion, 공통 bootstrap, 디렉터리 소유권을 설정합니다.'}
      </small>
    </div>
  );
}
