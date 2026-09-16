'use client';

import { useMemo, useState } from 'react';
import { AlertTriangle, Check, Link2, WandSparkles } from 'lucide-react';
import Choice from '../topology/Choice';
import type { Artifact } from '../../lib/api/delivery';
import type { TopologyNode } from '../topology/model';
import {
  analyzeArtifactMappings,
  compatibleArtifacts,
  recommendedAssignments,
  type ArtifactAssignments,
  type ArtifactRequirement,
} from './artifact-mapping';

const EMPTY = '__unmapped__';

const statusLabels: Record<ArtifactRequirement['status'], string> = {
  resolved: '연결됨',
  missing: '선택 필요',
  unresolved: '참조 없음',
  'kind-mismatch': '종류 불일치',
};

export default function ArtifactMappingPanel({
  nodes,
  artifacts,
  disabled,
  onApply,
}: {
  nodes: TopologyNode[];
  artifacts: Artifact[];
  disabled: boolean;
  onApply: (assignments: ArtifactAssignments) => number;
}) {
  const requirements = useMemo(
    () => analyzeArtifactMappings(nodes, artifacts),
    [nodes, artifacts],
  );
  const [assignments, setAssignments] = useState<ArtifactAssignments>({});
  const [message, setMessage] = useState('');

  const unresolved = requirements.filter(
    (requirement) => requirement.status !== 'resolved',
  ).length;
  const suggested = requirements.filter(
    (requirement) => requirement.suggestedArtifactId,
  ).length;
  const groups = Array.from(
    requirements
      .reduce((grouped, requirement) => {
        const group = grouped.get(requirement.nodeId) ?? [];
        group.push(requirement);
        grouped.set(requirement.nodeId, group);
        return grouped;
      }, new Map<string, ArtifactRequirement[]>())
      .entries(),
  );

  function apply() {
    try {
      const currentAssignments = Object.fromEntries(
        requirements.map((requirement) => [
          requirement.id,
          assignments[requirement.id] ??
            (requirement.status === 'resolved' ? requirement.currentRef : ''),
        ]),
      );
      const applied = onApply(currentAssignments);
      setMessage(
        applied
          ? `${applied}개 참조를 적용했습니다. 토폴로지에서 설계를 저장하세요.`
          : '변경할 artifact 참조가 없습니다.',
      );
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : '매핑 적용에 실패했습니다.',
      );
    }
  }

  return (
    <section className="mapping-card">
      <div className="section-heading mapping-heading">
        <span className="artifact-symbol">
          <Link2 size={19} />
        </span>
        <div>
          <h2>토폴로지 파일 매핑</h2>
          <p>
            배포 모델의 artifactRef를 현재 프로젝트 파일에 명시적으로
            연결합니다.
          </p>
        </div>
        {!!requirements.length && (
          <span
            className={
              unresolved ? 'mapping-summary warning' : 'mapping-summary'
            }
          >
            {requirements.length - unresolved}/{requirements.length} 연결
          </span>
        )}
      </div>

      {!requirements.length ? (
        <div className="empty-panel mapping-empty">
          토폴로지에 artifact가 필요한 Java 또는 서버 파일 설정이 없습니다.
        </div>
      ) : (
        <div className="mapping-groups">
          {groups.map(([nodeId, nodeRequirements]) => (
            <section className="mapping-group" key={nodeId}>
              <div className="mapping-node-heading">
                <strong>{nodeRequirements[0].nodeName}</strong>
                <span>{nodeRequirements[0].nodeKind}</span>
              </div>
              {nodeRequirements.map((requirement) => {
                const compatible = compatibleArtifacts(requirement, artifacts);
                const selected =
                  assignments[requirement.id] ??
                  (requirement.status === 'resolved'
                    ? requirement.currentRef
                    : EMPTY);
                return (
                  <div className="mapping-row" key={requirement.id}>
                    <div className="mapping-role">
                      <strong>{requirement.label}</strong>
                      <code title={requirement.fieldPath}>
                        {requirement.fieldPath}
                      </code>
                    </div>
                    <Choice
                      label={`${requirement.nodeName} ${requirement.label}`}
                      value={
                        compatible.some((artifact) => artifact.id === selected)
                          ? selected
                          : EMPTY
                      }
                      options={[
                        { label: '선택 필요', value: EMPTY },
                        ...compatible.map((artifact) => ({
                          label: artifact.originalName,
                          value: artifact.id,
                          description: `${artifact.kind} · ${shortArtifactId(artifact.id)}`,
                        })),
                      ]}
                      onChange={(artifactId) =>
                        setAssignments((current) => ({
                          ...current,
                          [requirement.id]:
                            artifactId === EMPTY ? '' : artifactId,
                        }))
                      }
                    />
                    <span className={`mapping-status ${requirement.status}`}>
                      {requirement.status === 'resolved' ? (
                        <Check size={13} />
                      ) : (
                        <AlertTriangle size={13} />
                      )}
                      {statusLabels[requirement.status]}
                    </span>
                  </div>
                );
              })}
            </section>
          ))}
        </div>
      )}

      {!!requirements.length && (
        <div className="mapping-actions">
          <button
            disabled={disabled || !suggested}
            onClick={() => {
              setAssignments((current) => ({
                ...current,
                ...recommendedAssignments(requirements),
              }));
              setMessage(
                suggested
                  ? `${suggested}개 추천을 선택했습니다. 내용을 확인한 뒤 적용하세요.`
                  : '확실하게 추천할 수 있는 파일이 없습니다.',
              );
            }}
          >
            <WandSparkles size={15} /> 추천 선택
          </button>
          <button className="primary" disabled={disabled} onClick={apply}>
            <Link2 size={15} /> 매핑 적용
          </button>
          {message && <output>{message}</output>}
        </div>
      )}
    </section>
  );
}

function shortArtifactId(id: string) {
  return id.length > 15 ? `${id.slice(0, 15)}…` : id;
}
