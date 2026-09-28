'use client';

import {
  AlertTriangle,
  Check,
  Copy,
  Download,
  FileCode,
  Package,
  Play,
} from 'lucide-react';
import {
  NativeSelect,
  NativeSelectOption,
} from '../../components/ui/native-select';
import type { GeneratorID } from '../../lib/api/contracts';
import GenerationHistory from './GenerationHistory';
import { REQUIRED_CHART_FORMAT } from './package-contract';
import {
  usePackageGeneration,
  type PackageGenerationInput,
} from './usePackageGeneration';
import { usePackagePreview } from './usePackagePreview';

const generationStatus = {
  queued: '작업 대기 중',
  running: '패키지 생성 중',
  succeeded: '생성 완료',
  failed: '생성 실패',
};

export default function ServerPackage(
  props: PackageGenerationInput & { onBack: () => void },
) {
  const { projectId, environment, revision, onBack } = props;
  const {
    capabilities,
    validation,
    generation,
    generatedPackage,
    selectedId,
    setSelectedId,
    busy,
    message,
    setMessage,
    generatorID,
    selectGenerator,
    selected,
    blockedReason,
    capabilityBlockedReason,
    generate,
    resumeGeneration,
    retryGeneration,
    download,
  } = usePackageGeneration(props);
  const {
    content,
    truncated,
    error: previewError,
  } = usePackagePreview(
    projectId,
    generatedPackage?.id,
    selected?.previewable ? selected.id : undefined,
  );

  async function copy() {
    try {
      await navigator.clipboard.writeText(content);
      setMessage('파일 내용을 클립보드에 복사했습니다.');
    } catch {
      setMessage('클립보드 권한이 없습니다. ZIP 다운로드를 이용하세요.');
    }
  }

  return (
    <section className="package-view">
      <div className="package-heading">
        <div>
          <span className="eyebrow">IMMUTABLE DELIVERY PACKAGE</span>
          <h2>
            <Package /> Crosswalk 배포 패키지
          </h2>
        </div>
        <div className="actions">
          <NativeSelect
            aria-label="패키지 생성기"
            value={generatorID}
            disabled={busy}
            onChange={(event) =>
              selectGenerator(event.target.value as GeneratorID)
            }
          >
            <NativeSelectOption value="crosswalk-vm">
              Crosswalk VM
            </NativeSelectOption>
            <NativeSelectOption value="crosswalk-docker">
              Crosswalk Docker
            </NativeSelectOption>
            <NativeSelectOption value="crosswalk-compose">
              Crosswalk Compose
            </NativeSelectOption>
          </NativeSelect>
          <button onClick={onBack}>편집기로 돌아가기</button>
          <button
            className={generatedPackage ? undefined : 'primary'}
            disabled={!!blockedReason || busy || !capabilities}
            onClick={() => void generate()}
          >
            <Play size={16} />{' '}
            {busy
              ? '처리 중…'
              : generatedPackage
                ? '다시 생성'
                : '검증 후 생성'}
          </button>
          <button
            className={generatedPackage ? 'primary' : undefined}
            disabled={!generatedPackage || busy}
            onClick={() => void download()}
          >
            <Download size={16} /> 실제 ZIP 다운로드
          </button>
        </div>
      </div>
      <div className={blockedReason ? 'draft-notice' : 'package-status'}>
        {blockedReason ||
          generatorID +
            ' · ' +
            REQUIRED_CHART_FORMAT +
            ' · ' +
            environment +
            ' · topology revision ' +
            revision +
            (generation
              ? ` · ${generationStatus[generation.status]} · ${generation.attempts}차 시도`
              : '')}
      </div>
      {generatedPackage && (
        <dl
          className="package-contract-info"
          aria-label="생성 패키지 계약 정보"
        >
          <div>
            <dt>차트 포맷</dt>
            <dd>
              <code>{generatedPackage.chartFormat}</code>
            </dd>
          </div>
          <div>
            <dt>생성기</dt>
            <dd>
              <code>
                {generatedPackage.generator}@{generatedPackage.generatorVersion}
              </code>
            </dd>
          </div>
          <div>
            <dt>토폴로지 스냅샷</dt>
            <dd>
              {generatedPackage.environment.toUpperCase()} · revision{' '}
              {generatedPackage.revision}
            </dd>
          </div>
        </dl>
      )}
      {validation && !validation.valid && (
        <div className="package-issues" role="alert">
          {validation.issues.map((issue, index) => (
            <p key={issue.code + ':' + index}>
              <AlertTriangle size={15} /> <strong>{issue.code}</strong>{' '}
              {issue.message} <code>{issue.path}</code>
            </p>
          ))}
        </div>
      )}
      {generatedPackage ? (
        <div className="file-browser">
          <aside>
            {generatedPackage.files.map((file) => (
              <button
                key={file.id}
                className={selectedId === file.id ? 'selected' : ''}
                onClick={() => setSelectedId(file.id)}
              >
                <FileCode size={16} /> {file.path}
              </button>
            ))}
          </aside>
          <div className="code-pane">
            <div className="code-header">
              <span>{selected?.path}</span>
              {selected?.previewable && (
                <button
                  disabled={!content || !!previewError}
                  onClick={() => void copy()}
                >
                  <Copy size={14} /> 복사
                </button>
              )}
            </div>
            {selected?.previewable ? (
              <>
                {truncated && (
                  <div className="draft-notice">
                    미리보기 크기 제한으로 일부 내용만 표시합니다. ZIP에는 전체
                    파일이 포함됩니다.
                  </div>
                )}
                <pre>
                  {previewError || content || '미리보기를 불러오는 중…'}
                </pre>
              </>
            ) : (
              <div className="binary-file">
                <Package size={34} />
                <strong>{selected?.path}</strong>
                <span>
                  {selected?.mediaType} · {selected?.sizeBytes.toLocaleString()}{' '}
                  bytes
                </span>
                <code>sha256:{selected?.sha256}</code>
                <p>
                  바이너리 파일은 미리보기 대신 실제 ZIP에서 확인할 수 있습니다.
                </p>
              </div>
            )}
          </div>
        </div>
      ) : (
        <div className="empty-panel package-empty">
          <Package size={32} />
          <strong>아직 생성된 패키지가 없습니다.</strong>
          <span>
            저장된 revision을 배포 목적으로 검증한 뒤 deploy.yml과 실제 artifact
            ZIP을 생성합니다.
          </span>
        </div>
      )}
      <output className="feedback">
        {validation?.valid ? <Check size={15} /> : <AlertTriangle size={15} />}
        {message ||
          '생성 요청 전 Blueprint와 선택한 Crosswalk 생성기 capability를 확인합니다.'}
      </output>
      {projectId && (
        <GenerationHistory
          key={projectId + ':' + environment}
          projectId={projectId}
          environment={environment}
          activeGeneration={generation}
          onResume={resumeGeneration}
          onRetry={retryGeneration}
          generationBlockedReason={capabilityBlockedReason}
          operationBusy={busy}
        />
      )}
    </section>
  );
}
