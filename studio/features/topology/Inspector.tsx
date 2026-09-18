'use client';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet';
import { Input } from '@/components/ui/input';
import { Trash2 } from 'lucide-react';
import { canPlaceOn, isWorkload, isTarget } from './placement';
import Choice from './Choice';
import { type TopologyNode, type TopologyEdge, type NodeData } from './model';
import DeploymentEditor from './DeploymentEditor';
import type { Artifact } from '../../lib/api/delivery';
export default function Inspector({
  node,
  edge,
  nodes,
  place,
  onClose,
  updateNode,
  remove,
  artifacts,
  errors = [],
}: {
  nodes: TopologyNode[];
  place: (id: string, targetId?: string) => void;
  node?: TopologyNode;
  edge?: TopologyEdge;
  onClose: () => void;
  updateNode: (data: Partial<NodeData>) => void;
  remove: () => void;
  artifacts: Artifact[];
  errors?: { path: string; message: string }[];
}) {
  type EditableField = Exclude<keyof NodeData, 'deployment'>;
  const errorFor = (key: EditableField) => {
    const suffix = (
      {
        label: '/name',
        host: '/host',
        heap: '/maxHeap',
        java: '/javaVersion',
        namespace: '/namespace',
        image: '/image',
        digest: '/digest',
        registryUsername: '/username',
        tokenEnv: '/tokenEnv',
        containerPort: '/containerPort',
      } as Record<string, string>
    )[key];
    return suffix
      ? errors.find(
          (error) =>
            error.path.endsWith(suffix) ||
            (key === 'host' && error.path.endsWith('/endpoint')),
        )
      : undefined;
  };
  const field = (label: string, key: EditableField, type = 'text') => (
    <label className="field">
      <span>{label}</span>
      <Input
        type={type}
        aria-invalid={!!errorFor(key)}
        value={String(node?.data[key] ?? '')}
        onChange={(e) =>
          updateNode({
            [key]: type === 'number' ? Number(e.target.value) : e.target.value,
          })
        }
      />
      {errorFor(key) && (
        <small style={{ color: '#b34e4e' }}>{errorFor(key)?.message}</small>
      )}
    </label>
  );
  return (
    <Sheet
      open={!!node || !!edge}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent className="inspector">
        <SheetHeader>
          <SheetTitle>{node?.data.label || '연결 설정'}</SheetTitle>
          <SheetDescription>
            {node
              ? '노드 설정은 토폴로지에 즉시 반영됩니다.'
              : '네트워크 정책은 설계 정보이며 방화벽을 변경하지 않습니다.'}
          </SheetDescription>
        </SheetHeader>
        <div className="inspector-fields">
          {errors.length > 0 && (
            <div role="alert" style={{ color: '#b34e4e', marginBottom: 16 }}>
              {errors.map((error, index) => (
                <p key={index}>{error.message}</p>
              ))}
            </div>
          )}
          {node && (
            <>
              {field('노드 이름', 'label')}
              {isWorkload(node) && (
                <div className="field">
                  <span>실행 대상</span>
                  <Choice
                    label="실행 대상"
                    value={node.parentId || 'unassigned'}
                    options={[
                      {
                        value: 'unassigned',
                        label:
                          node.data.kind === 'java' ||
                          node.data.kind === 'container'
                            ? '미배치'
                            : '외부 서비스',
                      },
                      ...nodes
                        .filter((candidate) => canPlaceOn(node, candidate))
                        .map((n) => ({ value: n.id, label: n.data.label })),
                    ]}
                    onChange={(value) =>
                      place(node.id, value === 'unassigned' ? undefined : value)
                    }
                  />
                </div>
              )}
              {node.data.kind === 'vm' && (
                <div className="field">
                  <span>호스트 유형</span>
                  <Choice
                    label="호스트 유형"
                    value={node.data.platform}
                    options={['On-prem VM', 'AWS EC2']}
                    onChange={(platform) => updateNode({ platform })}
                  />
                </div>
              )}
              {!node.parentId &&
                [
                  'vm',
                  'database',
                  'redis',
                  'elasticsearch',
                  'ingress',
                  'registry',
                ].includes(node.data.kind) &&
                field('호스트 / endpoint', 'host')}
              {['java', 'database', 'redis', 'elasticsearch'].includes(
                node.data.kind,
              ) && field('서비스 포트', 'port', 'number')}
              {node.data.kind === 'container' && (
                <>
                  {field('호스트 포트', 'port', 'number')}
                  {field('컨테이너 포트', 'containerPort', 'number')}
                  {field('이미지 repository · tag 제외', 'image')}
                  {field('이미지 digest · sha256', 'digest')}
                </>
              )}
              {node.data.kind === 'java' && (
                <>
                  <div className="field">
                    <span>Java version</span>
                    <Choice
                      label="Java version"
                      value={node.data.java}
                      options={['17', '21', '25']}
                      onChange={(java) => updateNode({ java })}
                    />
                  </div>
                  {field('최대 JVM heap', 'heap')}
                  <div className="field">
                    <span>실행 JAR artifact</span>
                    <Choice
                      label="실행 JAR artifact"
                      value={node.data.artifactRef || 'unassigned'}
                      options={[
                        { value: 'unassigned', label: '선택하지 않음' },
                        ...(node.data.artifactRef &&
                        !artifacts.some(
                          (artifact) => artifact.id === node.data.artifactRef,
                        )
                          ? [
                              {
                                value: node.data.artifactRef,
                                label:
                                  '찾을 수 없는 artifact · ' +
                                  node.data.artifactRef,
                              },
                            ]
                          : []),
                        ...artifacts
                          .filter((artifact) => artifact.kind === 'jar')
                          .map((artifact) => ({
                            value: artifact.id,
                            label: artifact.originalName,
                          })),
                      ]}
                      onChange={(value) => {
                        const artifact = artifacts.find(
                          (candidate) => candidate.id === value,
                        );
                        updateNode({
                          artifactRef: value === 'unassigned' ? null : value,
                          artifact: artifact?.originalName ?? '',
                        });
                      }}
                    />
                    {!artifacts.some((artifact) => artifact.kind === 'jar') && (
                      <small>파일 메뉴에서 JAR를 먼저 등록하세요.</small>
                    )}
                  </div>
                </>
              )}
              {['database', 'redis', 'elasticsearch'].includes(
                node.data.kind,
              ) && field('Secret 참조 이름 · 비밀 값 입력 금지', 'secretRef')}
              {node.data.kind === 'registry' && (
                <>
                  {field('Registry 사용자 이름', 'registryUsername')}
                  {field('Token 환경변수 이름 · 비밀 값 입력 금지', 'tokenEnv')}
                </>
              )}
              {node.data.kind === 'kubernetes' &&
                field('Namespace', 'namespace')}
              {['vm', 'java', 'container', 'redis', 'elasticsearch'].includes(
                node.data.kind,
              ) && (
                <DeploymentEditor
                  key={node.id}
                  kind={
                    node.data.kind as
                      | 'vm'
                      | 'java'
                      | 'container'
                      | 'redis'
                      | 'elasticsearch'
                  }
                  value={node.data.deployment}
                  onChange={(deployment) => updateNode({ deployment })}
                />
              )}
            </>
          )}
          {edge && (
            <>
              <div className="connection-pair">
                {edge.source} → {edge.target}
              </div>
              {edge.data?.port !== undefined ? (
                <p>
                  TCP ·{' '}
                  {nodes.find((node) => node.id === edge.target)?.data.port ??
                    edge.data.port}
                  <br />
                  포트는 대상 서비스 설정에서 변경합니다.
                  <br />
                  캔버스에서 포트 라벨과 선의 원형 굴곡점을 드래그해 겹침을
                  조정할 수 있습니다.
                </p>
              ) : (
                <p>이미지 발행 관계입니다.</p>
              )}
            </>
          )}
          <p className="field-note">
            변경 후 설계 저장을 누르세요. artifactRef에는 Studio에 등록된
            immutable artifact ID를 사용합니다.
          </p>
          <button className="danger-button" onClick={remove}>
            <Trash2 size={16} />{' '}
            {node
              ? isTarget(node)
                ? '실행 대상 삭제 · 내부 서비스 유지'
                : '노드 및 연결 삭제'
              : '연결 삭제'}
          </button>
        </div>
      </SheetContent>
    </Sheet>
  );
}
