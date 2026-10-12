import {
  ShieldCheck,
  FileCode,
  AlertTriangle,
  ChevronRight,
  Check,
} from 'lucide-react';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import type { Issue, TopologyNode } from '../topology/model';

export default function TopologyResults({
  tab,
  setTab,
  issues,
  nodes,
  serverValidated,
  topologyJSON,
  onSelectNode,
}: {
  tab: string;
  setTab: (value: string) => void;
  issues: Issue[];
  nodes: TopologyNode[];
  serverValidated: boolean;
  topologyJSON: string;
  onSelectNode: (nodeId: string) => void;
}) {
  const errors = issues.filter((issue) => issue.severity === 'error');
  return (
    <Tabs
      className="result-tabs"
      value={tab}
      onValueChange={(v) => setTab(String(v))}
    >
      <TabsList variant="line">
        <TabsTrigger value="issues">
          <ShieldCheck size={15} /> 검증 결과{' '}
          <span className={errors.length ? 'issue-count' : 'good-count'}>
            {issues.length}
          </span>
        </TabsTrigger>
        <TabsTrigger value="blueprint">
          <FileCode size={15} /> Blueprint
        </TabsTrigger>
      </TabsList>
      <TabsContent value="issues">
        <div className="issue-list">
          {issues.length ? (
            issues.map((issue) => (
              <button
                key={issue.id}
                className="issue-row"
                onClick={() => {
                  if (issue.nodeId) onSelectNode(issue.nodeId);
                }}
              >
                <AlertTriangle
                  size={15}
                  color={issue.severity === 'error' ? '#c45151' : '#ac862f'}
                />
                <span>{issue.message}</span>
                <small>
                  {nodes.find((n) => n.id === issue.nodeId)?.data.label}
                </small>
                <ChevronRight size={14} />
              </button>
            ))
          ) : (
            <div className="valid-state">
              <span>
                <Check size={20} />
              </span>
              <div>
                <strong>
                  {serverValidated
                    ? '서버 설계 검증을 통과했습니다.'
                    : '로컬 기본 검사 결과입니다. 서버 검증을 실행하세요.'}
                </strong>
                <p>
                  연결 규칙과 VM 포트 충돌 검사가 완료되었습니다. 실제 통신 가능
                  여부는 별도 확인이 필요합니다.
                </p>
              </div>
            </div>
          )}
        </div>
      </TabsContent>
      <TabsContent value="blueprint">
        <pre className="blueprint-code">{topologyJSON}</pre>
      </TabsContent>
    </Tabs>
  );
}
