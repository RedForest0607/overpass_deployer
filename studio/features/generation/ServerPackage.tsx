'use client';

import { useEffect, useRef, useState } from 'react';
import {
  AlertTriangle,
  Check,
  Copy,
  Download,
  FileCode,
  Package,
  Play,
} from 'lucide-react';
import type { Blueprint, Environment } from '../../lib/api/contracts';
import {
  deliveryAPI,
  type Capabilities,
  type GeneratedPackage,
  type Generation,
} from '../../lib/api/delivery';
import type { Validation } from '../../lib/api/projects';
import { downloadBlob } from '../../lib/download';
import {
  idempotencyAttempt,
  nextPollDelay,
  type IdempotencyAttempt,
} from './generation-state';

const wait = (milliseconds: number, signal: AbortSignal) =>
  new Promise<void>((resolve, reject) => {
    const timer = window.setTimeout(resolve, milliseconds);
    signal.addEventListener(
      'abort',
      () => {
        window.clearTimeout(timer);
        reject(new DOMException('Aborted', 'AbortError'));
      },
      { once: true },
    );
  });

const generationStatus = {
  queued: '작업 대기 중',
  running: '패키지 생성 중',
  succeeded: '생성 완료',
  failed: '생성 실패',
};

export default function ServerPackage({
  projectId,
  projectName,
  environment,
  revision,
  dirty,
  blueprint,
  onBack,
}: {
  projectId?: string;
  projectName: string;
  environment: Environment;
  revision: number;
  dirty: boolean;
  blueprint: Blueprint;
  onBack: () => void;
}) {
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null);
  const [validation, setValidation] = useState<Validation | null>(null);
  const [generation, setGeneration] = useState<Generation | null>(null);
  const [generatedPackage, setGeneratedPackage] =
    useState<GeneratedPackage | null>(null);
  const [selectedId, setSelectedId] = useState('');
  const [content, setContent] = useState('');
  const [truncated, setTruncated] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const operation = useRef<AbortController | null>(null);
  const retry = useRef<IdempotencyAttempt | null>(null);
  const signature = projectId + ':' + environment + ':' + revision + ':package';
  const capability = capabilities?.generators.find(
    (candidate) => candidate.id === 'crosswalk-vm',
  );
  const supported = !!capability?.outputs.includes('package');
  const selected = generatedPackage?.files.find(
    (file) => file.id === selectedId,
  );
  const blockedReason = !projectId
    ? '프로젝트를 먼저 선택하세요.'
    : dirty
      ? '미저장 변경이 있습니다. immutable revision을 만들기 위해 설계를 먼저 저장하세요.'
      : revision < 1
        ? '저장된 topology revision이 없습니다.'
        : capabilities && !supported
          ? '서버에서 crosswalk-vm package 생성을 지원하지 않습니다.'
          : '';

  useEffect(() => {
    const controller = new AbortController();
    deliveryAPI
      .capabilities(controller.signal)
      .then(setCapabilities)
      .catch((cause) => {
        if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
          setMessage(
            cause instanceof Error
              ? cause.message
              : '서버 기능을 확인하지 못했습니다.',
          );
        }
      });
    return () => controller.abort();
  }, []);

  useEffect(() => () => operation.current?.abort(), []);

  useEffect(() => {
    if (!projectId || !generatedPackage || !selected?.previewable) return;
    const controller = new AbortController();
    deliveryAPI
      .previewFile(
        projectId,
        generatedPackage.id,
        selected.id,
        controller.signal,
      )
      .then((preview) => {
        setContent(preview.content);
        setTruncated(preview.truncated);
      })
      .catch((cause) => {
        if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
          setMessage(
            cause instanceof Error
              ? cause.message
              : '파일 미리보기에 실패했습니다.',
          );
        }
      });
    return () => controller.abort();
  }, [projectId, generatedPackage, selected]);

  async function loadPackage(
    project: string,
    value: Generation,
    signal: AbortSignal,
  ) {
    let current = value;
    let pollAttempt = 1;
    while (current.status === 'queued' || current.status === 'running') {
      await wait(nextPollDelay(pollAttempt), signal);
      current = await deliveryAPI.getGeneration(project, current.id, signal);
      setGeneration(current);
      pollAttempt += 1;
    }
    if (current.status === 'failed') {
      throw new Error(current.error?.message || '패키지 생성에 실패했습니다.');
    }
    if (!current.packageId) {
      throw new Error('생성 결과에 package ID가 없습니다.');
    }
    const result = await deliveryAPI.getPackage(
      project,
      current.packageId,
      signal,
    );
    setGeneratedPackage(result);
    const deploy = result.files.find(
      (file) => file.path === 'deploy.yml' && file.previewable,
    );
    const first =
      deploy ??
      result.files.find((file) => file.previewable) ??
      result.files[0];
    setSelectedId(first?.id ?? '');
    retry.current = null;
    setMessage('revision ' + result.revision + ' 기반 패키지를 생성했습니다.');
  }

  async function generate() {
    if (!projectId || blockedReason || busy) return;
    operation.current?.abort();
    const controller = new AbortController();
    operation.current = controller;
    setBusy(true);
    setMessage('배포 목적 검증을 실행하고 있습니다…');
    setValidation(null);
    setGeneratedPackage(null);
    try {
      const checked = await deliveryAPI.validatePackage(
        projectId,
        environment,
        blueprint,
        crypto.randomUUID(),
      );
      if (controller.signal.aborted) return;
      setValidation(checked);
      if (!checked.valid) {
        setMessage('배포 목적 검증 오류를 해결한 뒤 다시 생성하세요.');
        return;
      }
      const attempt = idempotencyAttempt(retry.current, signature, () =>
        crypto.randomUUID(),
      );
      retry.current = attempt;
      setMessage('저장된 topology snapshot으로 패키지를 생성하고 있습니다…');
      const created = await deliveryAPI.createGeneration(
        projectId,
        environment,
        revision,
        'package',
        attempt.key,
      );
      if (controller.signal.aborted) return;
      setGeneration(created);
      await loadPackage(projectId, created, controller.signal);
    } catch (cause) {
      if (!(cause instanceof DOMException && cause.name === 'AbortError')) {
        setMessage(
          cause instanceof Error
            ? cause.message
            : '패키지 생성에 실패했습니다.',
        );
      }
    } finally {
      if (operation.current === controller) {
        operation.current = null;
        setBusy(false);
      }
    }
  }

  async function download() {
    if (!projectId || !generatedPackage) return;
    setBusy(true);
    try {
      const result = await deliveryAPI.downloadPackage(
        projectId,
        generatedPackage.id,
      );
      downloadBlob(
        result.blob,
        result.filename || projectName + '-' + environment + '.zip',
      );
      setMessage('실제 artifact가 포함된 서버 패키지를 다운로드했습니다.');
    } catch (cause) {
      setMessage(
        cause instanceof Error
          ? cause.message
          : '패키지 다운로드에 실패했습니다.',
      );
    } finally {
      setBusy(false);
    }
  }

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
            <Package /> Crosswalk VM 배포 패키지
          </h2>
        </div>
        <div className="actions">
          <button onClick={onBack}>편집기로 돌아가기</button>
          <button
            className="primary"
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
            disabled={!generatedPackage || busy}
            onClick={() => void download()}
          >
            <Download size={16} /> 실제 ZIP 다운로드
          </button>
        </div>
      </div>
      <div className={blockedReason ? 'draft-notice' : 'package-status'}>
        {blockedReason ||
          'crosswalk-vm · ' +
            environment +
            ' · topology revision ' +
            revision +
            (generation
              ? ` · ${generationStatus[generation.status]} · ${generation.attempts}차 시도`
              : '')}
      </div>
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
                <button onClick={() => void copy()}>
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
                <pre>{content || '미리보기를 불러오는 중…'}</pre>
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
          '생성 요청 전 프로젝트 artifact 참조와 Crosswalk VM capability를 확인합니다.'}
      </output>
    </section>
  );
}
