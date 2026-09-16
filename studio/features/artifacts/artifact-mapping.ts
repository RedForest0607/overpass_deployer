import type {
  ArtifactDelivery,
  ConfigArtifactDelivery,
  JavaDeployment,
  UtilityDeployment,
  VMDeployment,
} from '../../lib/api/contracts.ts';
import type { Artifact, ArtifactKind } from '../../lib/api/delivery.ts';
import type { TopologyNode } from '../topology/model.ts';

export type ArtifactMappingStatus =
  | 'resolved'
  | 'missing'
  | 'unresolved'
  | 'kind-mismatch';

export type ArtifactRequirement = {
  id: string;
  nodeId: string;
  nodeName: string;
  nodeKind: string;
  label: string;
  fieldPath: string;
  path: (string | number)[];
  currentRef: string;
  remotePath?: string;
  expectedKinds: ArtifactKind[];
  requiresExtract: boolean;
  status: ArtifactMappingStatus;
  artifact?: Artifact;
  suggestedArtifactId?: string;
};

export type ArtifactAssignments = Record<string, string>;

const CONFIG_KINDS: ArtifactKind[] = ['config-template', 'env-example', 'file'];
const FILE_KINDS: ArtifactKind[] = [
  'file',
  'archive',
  'config-template',
  'env-example',
];

function deliveryKinds(delivery: ArtifactDelivery): ArtifactKind[] {
  return delivery.extract?.enabled ? ['archive'] : FILE_KINDS;
}

function isCrosswalkExtractableArchive(name: string) {
  const lower = name.toLowerCase();
  return (
    lower.endsWith('.tar') ||
    lower.endsWith('.tgz') ||
    lower.endsWith('.tar.gz')
  );
}

function isCompatible(
  requirement: Pick<ArtifactRequirement, 'expectedKinds' | 'requiresExtract'>,
  artifact: Artifact,
) {
  return (
    requirement.expectedKinds.includes(artifact.kind) &&
    (!requirement.requiresExtract ||
      isCrosswalkExtractableArchive(artifact.originalName))
  );
}

function pathText(path: (string | number)[]) {
  return path.reduce<string>(
    (value, part) =>
      value + (typeof part === 'number' ? `[${part}]` : `.${part}`),
    'data',
  );
}

function filename(path?: string) {
  return path?.split('/').filter(Boolean).at(-1) ?? '';
}

function tokens(value: string) {
  return value
    .toLowerCase()
    .replace(/\.[a-z0-9.]+$/i, '')
    .split(/[^a-z0-9가-힣]+/)
    .filter(
      (token) => token.length >= 2 && !['artifact', 'replace'].includes(token),
    );
}

function suggestionScore(
  requirement: Omit<
    ArtifactRequirement,
    'status' | 'artifact' | 'suggestedArtifactId'
  >,
  artifact: Artifact,
) {
  if (!isCompatible(requirement, artifact)) return -1;
  const artifactName = artifact.originalName.toLowerCase();
  const remoteName = filename(requirement.remotePath).toLowerCase();
  let score = 1;
  if (remoteName && remoteName === artifactName) score += 100;
  for (const token of new Set([
    ...tokens(requirement.nodeName),
    ...tokens(requirement.currentRef),
  ])) {
    if (artifactName.includes(token)) score += 15;
  }
  if (requirement.expectedKinds.length === 1) score += 5;
  return score;
}

function suggestArtifact(
  requirement: Omit<
    ArtifactRequirement,
    'status' | 'artifact' | 'suggestedArtifactId'
  >,
  artifacts: Artifact[],
) {
  const ranked = artifacts
    .map((artifact) => ({
      artifact,
      score: suggestionScore(requirement, artifact),
    }))
    .filter(({ score }) => score >= 20)
    .sort((left, right) => right.score - left.score);
  if (!ranked.length || ranked[0].score === ranked[1]?.score) return undefined;
  return ranked[0].artifact.id;
}

function collectDeliveries(
  add: (
    label: string,
    path: (string | number)[],
    ref: string | undefined,
    kinds: ArtifactKind[],
    remotePath?: string,
    requiresExtract?: boolean,
  ) => void,
  deliveries: ArtifactDelivery[] | ConfigArtifactDelivery[] | undefined,
  basePath: (string | number)[],
  labelPrefix: string,
) {
  deliveries?.forEach((delivery, index) => {
    const artifactDelivery = delivery as ArtifactDelivery;
    add(
      `${labelPrefix} · ${filename(delivery.remotePath) || index + 1}`,
      [...basePath, index, 'artifactRef'],
      delivery.artifactRef,
      'extract' in artifactDelivery
        ? deliveryKinds(artifactDelivery)
        : CONFIG_KINDS,
      delivery.remotePath,
      artifactDelivery.extract?.enabled,
    );
  });
}

export function analyzeArtifactMappings(
  nodes: TopologyNode[],
  artifacts: Artifact[],
): ArtifactRequirement[] {
  const artifactById = new Map(
    artifacts.map((artifact) => [artifact.id, artifact]),
  );
  const requirements: ArtifactRequirement[] = [];

  for (const node of nodes) {
    const add = (
      label: string,
      path: (string | number)[],
      ref: string | null | undefined,
      expectedKinds: ArtifactKind[],
      remotePath?: string,
      requiresExtract = false,
    ) => {
      const currentRef = ref?.trim() ?? '';
      const artifact = currentRef ? artifactById.get(currentRef) : undefined;
      const base = {
        id: `${node.id}:${pathText(path)}`,
        nodeId: node.id,
        nodeName: node.data.label,
        nodeKind: node.data.kind,
        label,
        fieldPath: pathText(path),
        path,
        currentRef,
        remotePath,
        expectedKinds,
        requiresExtract,
      };
      const status: ArtifactMappingStatus = !currentRef
        ? 'missing'
        : !artifact
          ? 'unresolved'
          : isCompatible(base, artifact)
            ? 'resolved'
            : 'kind-mismatch';
      requirements.push({
        ...base,
        status,
        artifact,
        suggestedArtifactId:
          status === 'resolved' ? undefined : suggestArtifact(base, artifacts),
      });
    };

    if (node.data.kind === 'java') {
      const deployment = node.data.deployment as JavaDeployment | undefined;
      add('실행 JAR', ['artifactRef'], node.data.artifactRef, ['jar']);
      collectDeliveries(
        add,
        deployment?.configFiles,
        ['deployment', 'configFiles'],
        '설정 파일',
      );
      collectDeliveries(
        add,
        deployment?.extraFiles,
        ['deployment', 'extraFiles'],
        '추가 파일',
      );
      if (
        deployment?.script?.mode === 'local-file' ||
        deployment?.script?.artifactRef
      ) {
        add(
          '실행 스크립트',
          ['deployment', 'script', 'artifactRef'],
          deployment.script.artifactRef,
          ['file'],
          deployment.script.remotePath,
        );
      }
      if (deployment?.script?.valuesArtifactRef !== undefined) {
        add(
          '스크립트 values',
          ['deployment', 'script', 'valuesArtifactRef'],
          deployment.script.valuesArtifactRef,
          ['script-values'],
          deployment.script.valuesPackagePath,
        );
      }
      continue;
    }

    if (node.data.kind === 'vm') {
      const deployment = node.data.deployment as VMDeployment | undefined;
      collectDeliveries(
        add,
        deployment?.files,
        ['deployment', 'files'],
        '서버 파일',
      );
      deployment?.utilities?.forEach((utility, utilityIndex) => {
        collectDeliveries(
          add,
          utility.files,
          ['deployment', 'utilities', utilityIndex, 'files'],
          `${utility.name} 파일`,
        );
      });
      continue;
    }

    if (node.data.kind === 'redis' || node.data.kind === 'elasticsearch') {
      const deployment = node.data.deployment as UtilityDeployment | undefined;
      collectDeliveries(
        add,
        deployment?.files,
        ['deployment', 'files'],
        '유틸리티 파일',
      );
    }
  }

  return requirements;
}

export function compatibleArtifacts(
  requirement: ArtifactRequirement,
  artifacts: Artifact[],
) {
  return artifacts.filter((artifact) => isCompatible(requirement, artifact));
}

export function recommendedAssignments(requirements: ArtifactRequirement[]) {
  return Object.fromEntries(
    requirements
      .filter((requirement) => requirement.suggestedArtifactId)
      .map((requirement) => [requirement.id, requirement.suggestedArtifactId!]),
  );
}

function setAtPath(
  data: Record<string, unknown>,
  path: (string | number)[],
  value: string,
) {
  let target: unknown = data;
  for (const part of path.slice(0, -1)) {
    if (typeof target !== 'object' || target === null || !(part in target)) {
      throw new Error(`매핑 대상 경로가 변경되었습니다: ${pathText(path)}`);
    }
    target = (target as Record<string | number, unknown>)[part];
  }
  if (typeof target !== 'object' || target === null) {
    throw new Error(`매핑 대상 경로가 변경되었습니다: ${pathText(path)}`);
  }
  (target as Record<string | number, unknown>)[path.at(-1)!] = value;
}

export function applyArtifactMappings(
  nodes: TopologyNode[],
  artifacts: Artifact[],
  assignments: ArtifactAssignments,
) {
  const requirements = new Map(
    analyzeArtifactMappings(nodes, artifacts).map((requirement) => [
      requirement.id,
      requirement,
    ]),
  );
  const artifactsById = new Map(
    artifacts.map((artifact) => [artifact.id, artifact]),
  );
  const changes = new Map<
    string,
    { requirement: ArtifactRequirement; artifact: Artifact }[]
  >();

  for (const [requirementId, artifactId] of Object.entries(assignments)) {
    if (!artifactId) continue;
    const requirement = requirements.get(requirementId);
    if (!requirement)
      throw new Error(`알 수 없는 매핑 항목입니다: ${requirementId}`);
    const artifact = artifactsById.get(artifactId);
    if (!artifact)
      throw new Error(`현재 프로젝트의 artifact가 아닙니다: ${artifactId}`);
    if (!isCompatible(requirement, artifact)) {
      throw new Error(
        `${requirement.label}에는 ${artifact.kind} artifact ${artifact.originalName}을 사용할 수 없습니다.`,
      );
    }
    if (requirement.currentRef === artifact.id) continue;
    const nodeChanges = changes.get(requirement.nodeId) ?? [];
    nodeChanges.push({ requirement, artifact });
    changes.set(requirement.nodeId, nodeChanges);
  }

  let applied = 0;
  const mappedNodes = nodes.map((node) => {
    const nodeChanges = changes.get(node.id);
    if (!nodeChanges?.length) return node;
    const data = structuredClone(node.data) as unknown as Record<
      string,
      unknown
    >;
    for (const { requirement, artifact } of nodeChanges) {
      setAtPath(data, requirement.path, artifact.id);
      if (
        requirement.path.length === 1 &&
        requirement.path[0] === 'artifactRef'
      ) {
        data.artifact = artifact.originalName;
      }
      applied += 1;
    }
    return { ...node, data: data as unknown as TopologyNode['data'] };
  });

  return { nodes: mappedNodes, applied };
}
