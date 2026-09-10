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
import { isWorkload, isTarget } from './placement';
import Choice from './Choice';
import { type TopologyNode, type TopologyEdge, type NodeData } from './model';
export default function Inspector({
  node,
  edge,
  nodes,
  place,
  onClose,
  updateNode,
  updateEdge,
  remove,
  errors = [],
}: {
  nodes: TopologyNode[];
  place: (id: string, targetId?: string) => void;
  node?: TopologyNode;
  edge?: TopologyEdge;
  onClose: () => void;
  updateNode: (data: Partial<NodeData>) => void;
  updateEdge: (data: NonNullable<TopologyEdge['data']>) => void;
  remove: () => void;
  errors?: { path: string; message: string }[];
}) {
  const errorFor = (key: keyof NodeData) => {
    const suffix = (
      {
        label: '/name',
        host: '/host',
        heap: '/maxHeap',
        java: '/javaVersion',
        namespace: '/namespace',
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
  const field = (label: string, key: keyof NodeData, type = 'text') => (
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
                <label className="field">
                  <span>실행 대상</span>
                  <Choice
                    label="실행 대상"
                    value={node.parentId || 'unassigned'}
                    options={[
                      {
                        value: 'unassigned',
                        label:
                          node.data.kind === 'java' ? '미배치' : '외부 서비스',
                      },
                      ...nodes
                        .filter(isTarget)
                        .map((n) => ({ value: n.id, label: n.data.label })),
                    ]}
                    onChange={(value) =>
                      place(node.id, value === 'unassigned' ? undefined : value)
                    }
                  />
                </label>
              )}
              {node.data.kind === 'vm' && (
                <label className="field">
                  <span>호스트 유형</span>
                  <Choice
                    label="호스트 유형"
                    value={node.data.platform}
                    options={['On-prem VM', 'AWS EC2']}
                    onChange={(platform) => updateNode({ platform })}
                  />
                </label>
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
              {node.data.kind === 'java' && (
                <>
                  <label className="field">
                    <span>Java version</span>
                    <Choice
                      label="Java version"
                      value={node.data.java}
                      options={['17', '21', '25']}
                      onChange={(java) => updateNode({ java })}
                    />
                  </label>
                  {field('최대 JVM heap', 'heap')}
                  {field('JAR 파일명 (실제 업로드는 후속 연결)', 'artifact')}
                </>
              )}
              {['database', 'redis', 'elasticsearch', 'registry'].includes(
                node.data.kind,
              ) && field('Secret 참조 이름 · 비밀 값 입력 금지', 'secretRef')}
              {node.data.kind === 'kubernetes' &&
                field('Namespace', 'namespace')}
            </>
          )}
          {edge && (
            <>
              <div className="connection-pair">
                {edge.source} → {edge.target}
              </div>
              {edge.data ? (
                <p>
                  TCP ·{' '}
                  {nodes.find((node) => node.id === edge.target)?.data.port ??
                    edge.data.port}
                  <br />
                  포트는 대상 서비스 설정에서 변경합니다.
                </p>
              ) : (
                <p>이미지 발행 관계입니다.</p>
              )}
            </>
          )}
          <p className="field-note">
            변경 후 설계 저장을 누르세요. JAR 파일명은 로컬 초안용이며
            업로드·영속 저장은 아직 지원하지 않습니다.
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
