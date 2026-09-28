'use client';

import { useEffect, useRef, useState } from 'react';
import type {
  Blueprint,
  Environment,
  GeneratorID,
} from '../../lib/api/contracts';
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
  type IdempotencyAttempt,
} from './generation-state';
import {
  evaluatePackageCapability,
  DEFAULT_GENERATOR,
} from './package-contract';
import { resolveGeneratedPackage } from './package-polling';

export type PackageGenerationInput = {
  projectId?: string;
  projectName: string;
  environment: Environment;
  revision: number;
  dirty: boolean;
  blueprint: Blueprint;
};

export function usePackageGeneration({
  projectId,
  projectName,
  environment,
  revision,
  dirty,
  blueprint,
}: PackageGenerationInput) {
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null);
  const [validation, setValidation] = useState<Validation | null>(null);
  const [generation, setGeneration] = useState<Generation | null>(null);
  const [generatedPackage, setGeneratedPackage] =
    useState<GeneratedPackage | null>(null);
  const [selectedId, setSelectedId] = useState('');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [capabilityError, setCapabilityError] = useState('');
  const [generatorID, setGeneratorID] = useState<GeneratorID>(() =>
    blueprint.nodes.some((node) => node.kind === 'container')
      ? 'crosswalk-docker'
      : DEFAULT_GENERATOR,
  );
  const operation = useRef<AbortController | null>(null);
  const retry = useRef<IdempotencyAttempt | null>(null);
  const signature =
    projectId +
    ':' +
    environment +
    ':' +
    revision +
    ':' +
    generatorID +
    ':package';
  const packageCapability = evaluatePackageCapability(
    capabilities,
    generatorID,
  );
  const capabilityBlockedReason = capabilityError
    ? capabilityError
    : capabilities
      ? packageCapability.reason
      : '서버의 패키지 호환성을 확인하고 있습니다…';
  const selected = generatedPackage?.files.find(
    (file) => file.id === selectedId,
  );
  const blockedReason = !projectId
    ? '프로젝트를 먼저 선택하세요.'
    : dirty
      ? '미저장 변경이 있습니다. immutable revision을 만들기 위해 설계를 먼저 저장하세요.'
      : revision < 1
        ? '저장된 topology revision이 없습니다.'
        : capabilityBlockedReason
          ? capabilityBlockedReason
          : '';

  useEffect(() => {
    const controller = new AbortController();
    deliveryAPI
      .capabilities(controller.signal)
      .then((value) => {
        if (controller.signal.aborted) return;
        setCapabilities(value);
        setCapabilityError('');
      })
      .catch((cause) => {
        if (!controller.signal.aborted) {
          const error =
            cause instanceof Error
              ? cause.message
              : '서버 기능을 확인하지 못했습니다.';
          setCapabilityError(error);
          setMessage(error);
        }
      });
    return () => controller.abort();
  }, []);

  useEffect(
    () => () => {
      operation.current?.abort();
      operation.current = null;
    },
    [projectId, environment, revision],
  );

  async function loadPackage(
    project: string,
    value: Generation,
    signal: AbortSignal,
    expectedGenerator: GeneratorID,
  ) {
    const result = await resolveGeneratedPackage(
      deliveryAPI,
      project,
      value,
      signal,
      expectedGenerator,
      setGeneration,
    );
    signal.throwIfAborted();
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
    setMessage(
      `revision ${result.revision} · ${result.chartFormat} · ${result.generator}@${result.generatorVersion} 패키지를 생성했습니다.`,
    );
  }

  async function generate() {
    if (!projectId || blockedReason || operation.current) return;
    const controller = new AbortController();
    operation.current = controller;
    setBusy(true);
    setMessage('배포 목적 검증을 실행하고 있습니다…');
    setValidation(null);
    setGeneration(null);
    setGeneratedPackage(null);
    try {
      const checked = await deliveryAPI.validatePackage(
        projectId,
        environment,
        blueprint,
        crypto.randomUUID(),
        generatorID,
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
        generatorID,
      );
      if (controller.signal.aborted) return;
      setGeneration(created);
      await loadPackage(projectId, created, controller.signal, generatorID);
    } catch (cause) {
      if (!controller.signal.aborted) {
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

  async function resumeGeneration(value: Generation, retryKey?: string) {
    if (!projectId || operation.current) return;
    const controller = new AbortController();
    operation.current = controller;
    setBusy(true);
    setGeneration(value);
    setGeneratedPackage(null);
    setMessage(
      retryKey
        ? '실패한 snapshot을 다시 생성하고 있습니다…'
        : '선택한 생성 작업을 불러오고 있습니다…',
    );
    try {
      if (retryKey) {
        value = await deliveryAPI.retryGeneration(
          projectId,
          value.id,
          retryKey,
        );
        controller.signal.throwIfAborted();
        setGeneration(value);
      }
      const historyGenerator =
        value.generator === 'crosswalk-docker' ||
        value.generator === 'crosswalk-compose'
          ? value.generator
          : 'crosswalk-vm';
      setGeneratorID(historyGenerator);
      await loadPackage(projectId, value, controller.signal, historyGenerator);
    } finally {
      if (operation.current === controller) {
        operation.current = null;
        setBusy(false);
      }
    }
  }

  async function download() {
    if (!projectId || !generatedPackage || operation.current) return;
    const controller = new AbortController();
    operation.current = controller;
    setBusy(true);
    try {
      const result = await deliveryAPI.downloadPackage(
        projectId,
        generatedPackage.id,
        controller.signal,
      );
      controller.signal.throwIfAborted();
      downloadBlob(
        result.blob,
        result.filename || projectName + '-' + environment + '.zip',
      );
      setMessage('실제 artifact가 포함된 서버 패키지를 다운로드했습니다.');
    } catch (cause) {
      if (!controller.signal.aborted)
        setMessage(
          cause instanceof Error
            ? cause.message
            : '패키지 다운로드에 실패했습니다.',
        );
    } finally {
      if (operation.current === controller) {
        operation.current = null;
        setBusy(false);
      }
    }
  }

  function selectGenerator(value: GeneratorID) {
    if (operation.current) return;
    setGeneratorID(value);
    setGeneratedPackage(null);
    setGeneration(null);
    setValidation(null);
    setSelectedId('');
    setMessage('');
    retry.current = null;
  }

  return {
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
    retryGeneration: (value: Generation, key: string) =>
      resumeGeneration(value, key),
    download,
  };
}
