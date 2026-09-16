'use client';
import { useMemo, useState, useCallback } from 'react';
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  useReactFlow,
  type Connection,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  Boxes,
  Workflow,
  Package,
  ArrowUpRight,
  ShieldCheck,
  Plus,
  Folder,
  ChevronRight,
  CircleHelp,
  Check,
  AlertTriangle,
  FileCode,
  Download,
  FileUp,
  Undo2,
  FileArchive,
} from 'lucide-react';
import {
  SidebarProvider,
  Sidebar,
  SidebarContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from '@/components/ui/sidebar';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import {
  catalog,
  createNode,
  connectionType,
  validate,
  type Kind,
  type TopologyNode,
  type TopologyEdge,
} from '../topology/model';
import ResourceNode, { ResourceIcon } from '../topology/ResourceNode';
import TargetNode from '../topology/TargetNode';
import CommunicationEdge from '../topology/CommunicationEdge';
import {
  isTarget,
  isWorkload,
  placeWorkload,
  removeResource,
  layoutPlacement,
} from '../topology/placement';
import Inspector from '../topology/Inspector';
import Choice from '../topology/Choice';
import { downloadBlob } from '../../lib/download';
import ServerPackage from '../generation/ServerPackage';
import { filesFor } from '../generation/preview';
import ArtifactManager from '../artifacts/ArtifactManager';
import { useArtifacts } from '../artifacts/useArtifacts';
import { applyArtifactMappings } from '../artifacts/artifact-mapping';
import type { ArtifactAssignments } from '../artifacts/artifact-mapping';

import { usePersistence } from './usePersistence';
import { toAPI } from '../topology/api-adapter';
import { defaultDeploymentProfile } from '../topology/api-adapter';
import type { DeploymentProfile } from '../../lib/api/contracts';
import DeploymentProfileEditor from '../topology/DeploymentProfileEditor';
import TopologyImportDialog from '../topology/TopologyImportDialog';
import type { ImportedTopology } from '../topology/topology-import';
import ProjectsPanel from './ProjectsPanel';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/components/ui/alert-dialog';

const nodeTypes = { resource: ResourceNode, targetGroup: TargetNode };
const edgeTypes = { communication: CommunicationEdge };
export default function Studio() {
  return (
    <ReactFlowProvider>
      <Workspace />
    </ReactFlowProvider>
  );
}
function Workspace() {
  const initial = useMemo(
    () => ({ nodes: [] as TopologyNode[], edges: [] as TopologyEdge[] }),
    [],
  );
  const [nodes, setNodes, onNodesChange] = useNodesState<TopologyNode>(
    initial.nodes,
  );
  const [edges, setEdges, onEdgesChange] = useEdgesState<TopologyEdge>(
    initial.edges,
  );
  const [view, setView] = useState('projects');
  const [confirmReload, setConfirmReload] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [viewport, setViewport] = useState({ x: 0, y: 0, zoom: 1 });
  const [deployment, setDeployment] = useState<DeploymentProfile>(
    defaultDeploymentProfile,
  );
  const [selection, setSelection] = useState<{
    type: 'node' | 'edge';
    id: string;
  } | null>(null);
  const [message, setMessage] = useState('');
  const [tab, setTab] = useState('issues');
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [lastRemoval, setLastRemoval] = useState<{
    nodes: TopologyNode[];
    edges: TopologyEdge[];
  } | null>(null);
  const flow = useReactFlow();
  const persistence = usePersistence(
    { nodes, edges, viewport, deployment },
    (graph) => {
      setNodes(graph.nodes);
      setEdges(graph.edges);
      setDeployment(graph.deployment ?? defaultDeploymentProfile);
      setSelection(null);
      setLastRemoval(null);
      setView('topology');
      const restoredViewport = graph.viewport ?? { x: 0, y: 0, zoom: 1 };
      setViewport(restoredViewport);
      void flow.setViewport(restoredViewport);
    },
  );
  const project = persistence.active?.name ?? '프로젝트를 선택하세요';
  const environment = persistence.environment.toUpperCase();
  const artifacts = useArtifacts(persistence.active?.id);
  const localIssues = useMemo(() => validate(nodes, edges), [nodes, edges]);
  const issues = persistence.validation
    ? persistence.validation.issues.map((issue, index) => ({
        id: issue.code + index,
        nodeId: issue.nodeIds[0],
        severity: issue.severity,
        message: issue.message + ' (' + issue.path + ')',
      }))
    : localIssues;
  const errors = issues.filter((i) => i.severity === 'error');
  const topologyDocument = useMemo(
    () => toAPI(nodes, edges, viewport, deployment),
    [nodes, edges, viewport, deployment],
  );
  const files = useMemo(
    () => ({
      ...filesFor(nodes, edges, environment),
      'topology.json': JSON.stringify(
        {
          environment: persistence.environment,
          ...topologyDocument,
        },
        null,
        2,
      ),
    }),
    [nodes, edges, environment, persistence.environment, topologyDocument],
  );
  const visibleEdges = useMemo(
    () =>
      edges.filter(
        (edge) =>
          !nodes.some((node) => node.id === edge.target && isTarget(node)),
      ),
    [edges, nodes],
  );
  const updateEdgeLayout = useCallback(
    (edgeId: string, layout: NonNullable<TopologyEdge['data']>['layout']) => {
      const normalized =
        layout?.bendOffset || layout?.labelPosition !== undefined
          ? layout
          : undefined;
      setEdges((current) =>
        current.map((edge) =>
          edge.id === edgeId
            ? {
                ...edge,
                data: { ...(edge.data ?? {}), layout: normalized },
              }
            : edge,
        ),
      );
    },
    [setEdges],
  );
  const renderedEdges = useMemo(
    () =>
      visibleEdges.map((edge) => ({
        ...edge,
        data: {
          ...(edge.data ?? {}),
          onLayoutChange: (
            layout: NonNullable<TopologyEdge['data']>['layout'],
          ) => updateEdgeLayout(edge.id, layout),
          onSelect: () => setSelection({ type: 'edge', id: edge.id }),
        },
      })),
    [visibleEdges, updateEdgeLayout, setSelection],
  );
  function importTopology(value: ImportedTopology, filename?: string) {
    setNodes(value.graph.nodes);
    setEdges(value.graph.edges);
    setDeployment(value.graph.deployment ?? defaultDeploymentProfile);
    setViewport(value.graph.viewport);
    setSelection(null);
    setLastRemoval(null);
    setTab('issues');
    setMessage(
      `${filename ?? '입력한 JSON'}을 에디터에 적용했습니다. 검증 후 설계 저장을 누르세요.`,
    );
    void flow.setViewport(value.graph.viewport);
  }
  function place(id: string, targetId?: string) {
    const next = placeWorkload(nodes, edges, id, targetId);
    setNodes(next.nodes);
    setEdges(next.edges);
  }
  const selectedNode =
    selection?.type === 'node'
      ? nodes.find((n) => n.id === selection.id)
      : undefined;
  const selectedEdge =
    selection?.type === 'edge'
      ? edges.find((e) => e.id === selection.id)
      : undefined;
  const valid = useCallback(
    (c: Connection | TopologyEdge) => {
      const from = nodes.find((n) => n.id === c.source),
        to = nodes.find((n) => n.id === c.target);
      return (
        !!from &&
        !!to &&
        from.id !== to.id &&
        !!connectionType(from.data.kind, to.data.kind) &&
        !edges.some((e) => e.source === c.source && e.target === c.target)
      );
    },
    [nodes, edges],
  );
  function connect(c: Connection) {
    if (!valid(c)) {
      setMessage('이 노드 조합은 연결할 수 없습니다.');
      return;
    }
    const from = nodes.find((n) => n.id === c.source)!,
      to = nodes.find((n) => n.id === c.target)!;
    const relation = connectionType(from.data.kind, to.data.kind)!;
    if (relation === 'deploys_to') {
      place(from.id, to.id);
      return;
    }
    const network = ['connects_to', 'routes_to'].includes(relation);
    setEdges((es) =>
      addEdge(
        {
          ...c,
          type: 'communication',
          label: network ? 'TCP · ' + to.data.port : relation,
          data: network ? { protocol: 'TCP', port: to.data.port } : undefined,
        },
        es,
      ),
    );
  }
  function add(kind: Kind, position?: { x: number; y: number }) {
    const id = crypto.randomUUID();
    const node = createNode(
      kind,
      id,
      position?.x ?? 550,
      position?.y ?? 120 + (nodes.length % 3) * 190,
    );
    const targets = nodes.filter(isTarget);
    const target = position
      ? targets.find(
          (n) =>
            position.x >= n.position.x &&
            position.x <= n.position.x + Number(n.style?.width ?? 340) &&
            position.y >= n.position.y &&
            position.y <= n.position.y + Number(n.style?.height ?? 300),
        )
      : targets.length === 1
        ? targets[0]
        : undefined;
    const next =
      isWorkload(node) && target
        ? placeWorkload([...nodes, node], edges, id, target.id)
        : { nodes: layoutPlacement([...nodes, node]), edges };
    setNodes(next.nodes);
    setEdges(next.edges);
    setMessage('노드를 추가했습니다. 더블클릭해 설정하세요.');
    setPaletteOpen(false);
    setTimeout(() => flow.fitView({ padding: 0.25, duration: 250 }), 50);
  }
  function remove() {
    if (!selection) return;
    setLastRemoval({ nodes, edges });
    if (selection.type === 'node') {
      const next = removeResource(nodes, edges, selection.id);
      setNodes(next.nodes);
      setEdges(next.edges);
    } else setEdges((es) => es.filter((e) => e.id !== selection.id));
    setSelection(null);
    setMessage('삭제했습니다. 실행 취소로 복구할 수 있습니다.');
  }

  function preview() {
    if (!persistence.active) {
      setMessage('프로젝트를 먼저 선택하세요.');
      return;
    }
    setView('packages');
  }
  function mapArtifacts(
    assignments: ArtifactAssignments,
    availableArtifacts = artifacts.items,
  ) {
    const result = applyArtifactMappings(
      nodes,
      availableArtifacts,
      assignments,
    );
    if (result.applied) {
      setNodes(result.nodes);
      setMessage(
        `${result.applied}개 artifact 참조를 적용했습니다. 토폴로지에서 설계를 저장하세요.`,
      );
    }
    return result.applied;
  }
  return (
    <SidebarProvider>
      <div className="studio">
        <Sidebar collapsible="none" className="nav">
          <div className="brand">
            <Boxes /> overpass<span>STUDIO</span>
          </div>
          <SidebarContent>
            <small>WORKSPACE</small>
            <SidebarMenu>
              {[
                { id: 'projects', label: '프로젝트', Icon: Folder },
                { id: 'artifacts', label: '파일', Icon: FileArchive },
                { id: 'topology', label: '토폴로지', Icon: Workflow },
                { id: 'packages', label: '배포 패키지', Icon: Package },
              ].map(({ id, label, Icon }) => (
                <SidebarMenuItem key={id}>
                  <SidebarMenuButton
                    className="nav-item"
                    isActive={view === id}
                    onClick={() =>
                      id === 'packages' ? preview() : setView(id)
                    }
                  >
                    <Icon size={18} />
                    <span>{label}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
            <div className="nav-project">
              <small>CURRENT PROJECT</small>
              <p>
                <span className="project-dot" /> {project}
              </p>
              <span className="secondary">Java · Development</span>
            </div>
          </SidebarContent>
          <div className="nav-bottom">
            <div className="workspace-id">
              <span className="avatar">OP</span>
              <div>
                Platform team
                <br />
                <span>Local workspace</span>
              </div>
            </div>
            <p>로컬 저장 · 설계 모드</p>
          </div>
        </Sidebar>
        <main className="main">
          <header>
            <span>
              Workspace <ChevronRight size={13} /> <b>Commerce platform</b>{' '}
              <ChevronRight size={13} />{' '}
              {view === 'topology'
                ? 'Topology'
                : view === 'projects'
                  ? 'Projects'
                  : view === 'artifacts'
                    ? 'Artifacts'
                    : 'Delivery package'}
            </span>
            <span className="header-right">
              <span className="green-dot" />{' '}
              {persistence.busy
                ? '요청 중'
                : persistence.dirty
                  ? '미저장 변경'
                  : '로컬 DB · r' + persistence.revision}{' '}
              <span className="avatar">OP</span>
            </span>
          </header>
          <section className="page-heading">
            <div>
              <div className="eyebrow">DEPLOYMENT WORKSPACE</div>
              <h1>
                {view === 'projects'
                  ? '프로젝트'
                  : view === 'artifacts'
                    ? '프로젝트 파일'
                    : project}{' '}
                <span className="pill">{environment}</span>
              </h1>
              <p>
                {view === 'projects'
                  ? '프로젝트와 환경별 배포 구성을 저장하고 관리하세요.'
                  : view === 'artifacts'
                    ? 'JAR, 설정, 일반 파일과 배포 스크립트 입력값을 프로젝트 범위로 관리하세요.'
                    : '서비스의 연결부터 배포 구성까지, 하나의 작업공간에서.'}
              </p>
            </div>
            <div className="actions">
              <Choice
                label="환경"
                value={environment}
                options={['DEV', 'QA', 'PROD']}
                onChange={(value) => {
                  if (persistence.active)
                    void persistence.open(
                      persistence.active,
                      value.toLowerCase() as 'dev' | 'qa' | 'prod',
                    );
                }}
              />
              {view === 'topology' && (
                <>
                  <button
                    disabled={
                      persistence.busy ||
                      !persistence.active ||
                      persistence.conflict
                    }
                    onClick={() => {
                      setMessage('');
                      void persistence.save();
                    }}
                  >
                    설계 저장
                  </button>
                  <button
                    disabled={persistence.busy || !persistence.active}
                    onClick={() => setConfirmReload(true)}
                  >
                    서버 버전 다시 불러오기
                  </button>
                  <button
                    disabled={persistence.busy || !persistence.active}
                    onClick={() => {
                      setTab('issues');
                      setMessage('');
                      void persistence.validate();
                    }}
                  >
                    <ShieldCheck size={16} /> 서버 검증
                  </button>
                  <button className="primary" onClick={preview}>
                    <Package size={16} /> 산출물 미리보기{' '}
                    <ArrowUpRight size={16} />
                  </button>
                </>
              )}
            </div>
          </section>
          <output
            style={{
              display: 'block',
              margin: '0 30px 12px',
              color: persistence.conflict ? '#b34e4e' : '#397b60',
            }}
          >
            {persistence.notice}
          </output>
          {view === 'projects' ? (
            <ProjectsPanel
              projects={persistence.projects}
              busy={persistence.busy}
              open={(project) => {
                void persistence.open(project);
                setView('topology');
              }}
              edit={persistence.editProject}
              refresh={() => {
                void persistence.refresh();
              }}
            />
          ) : view === 'artifacts' ? (
            <ArtifactManager
              projectId={persistence.active?.id}
              projectName={project}
              artifacts={artifacts.items}
              busy={artifacts.busy}
              error={artifacts.error}
              upload={artifacts.upload}
              refresh={artifacts.refresh}
              readText={artifacts.readText}
              nodes={nodes}
              mapArtifacts={mapArtifacts}
            />
          ) : view === 'packages' ? (
            <ServerPackage
              key={
                (persistence.active?.id ?? 'none') +
                ':' +
                persistence.environment +
                ':' +
                persistence.revision
              }
              projectId={persistence.active?.id}
              projectName={project}
              environment={persistence.environment}
              revision={persistence.revision}
              dirty={persistence.dirty}
              blueprint={topologyDocument.blueprint}
              onBack={() => setView('topology')}
            />
          ) : (
            <>
              <div className="editor-toolbar">
                <div>
                  <Workflow size={16} />
                  <strong>Topology editor</strong>
                  <span className="toolbar-divider" />
                  <span>{nodes.length} nodes</span>
                  <span>{visibleEdges.length} connections</span>
                </div>
                <div>
                  <button
                    className="mobile-palette"
                    onClick={() => setPaletteOpen(!paletteOpen)}
                  >
                    <Plus size={14} /> 노드
                  </button>
                  <button
                    disabled={!lastRemoval}
                    onClick={() => {
                      if (lastRemoval) {
                        setNodes(lastRemoval.nodes);
                        setEdges(lastRemoval.edges);
                        setLastRemoval(null);
                        setMessage('삭제를 취소했습니다.');
                      }
                    }}
                  >
                    <Undo2 size={15} /> 삭제 취소
                  </button>
                  <button
                    disabled={persistence.busy || !persistence.active}
                    title="토폴로지 JSON 가져오기"
                    onClick={() => setImportOpen(true)}
                  >
                    <FileUp size={15} /> JSON 가져오기
                  </button>
                  <button
                    title="토폴로지 JSON 다운로드"
                    onClick={() =>
                      downloadBlob(
                        new Blob([files['topology.json']], {
                          type: 'application/json',
                        }),
                        project + '-topology.json',
                      )
                    }
                  >
                    <Download size={15} /> JSON
                  </button>
                </div>
              </div>
              <div
                className="editor"
                inert={persistence.busy || !persistence.active}
              >
                <aside className={'palette ' + (paletteOpen ? 'open' : '')}>
                  <h3>
                    노드 라이브러리 <span>{catalog.length}</span>
                  </h3>
                  <p className="palette-help">클릭 또는 캔버스로 드래그</p>
                  {Array.from(new Set(catalog.map((c) => c.group))).map(
                    (group) => (
                      <section key={group}>
                        <small>{group}</small>
                        {catalog
                          .filter((c) => c.group === group)
                          .map((c) => (
                            <button
                              key={c.kind}
                              className="palette-node"
                              draggable
                              onDragStart={(e) => {
                                e.dataTransfer.setData(
                                  'application/overpass-node',
                                  c.kind,
                                );
                                e.dataTransfer.effectAllowed = 'move';
                              }}
                              onClick={() => add(c.kind)}
                            >
                              <span style={{ color: c.color }}>
                                <ResourceIcon kind={c.kind} size={18} />
                              </span>
                              <span>
                                <strong>{c.title}</strong>
                                <small>{c.subtitle}</small>
                              </span>
                              <Plus size={13} />
                            </button>
                          ))}
                      </section>
                    ),
                  )}
                  <div className="palette-note">
                    <CircleHelp size={16} />
                    <span>
                      Java·Redis·Elasticsearch는 VM 내부로 드래그하세요. 실행
                      대상은 설정에서 변경하며, 핸들은 통신 연결에 사용합니다.
                    </span>
                  </div>
                  <DeploymentProfileEditor
                    value={deployment}
                    onChange={setDeployment}
                  />
                </aside>
                <section className="canvas-stack">
                  <div
                    className="flow-container"
                    onDragOver={(e) => {
                      e.preventDefault();
                      e.dataTransfer.dropEffect = 'move';
                    }}
                    onDrop={(e) => {
                      e.preventDefault();
                      const kind = e.dataTransfer.getData(
                        'application/overpass-node',
                      ) as Kind;
                      if (catalog.some((c) => c.kind === kind))
                        add(
                          kind,
                          flow.screenToFlowPosition({
                            x: e.clientX,
                            y: e.clientY,
                          }),
                        );
                    }}
                  >
                    <ReactFlow
                      nodes={nodes}
                      edges={renderedEdges}
                      onNodesChange={onNodesChange}
                      onMoveEnd={(_, value) => setViewport(value)}
                      onEdgesChange={onEdgesChange}
                      onConnect={connect}
                      isValidConnection={valid}
                      nodeTypes={nodeTypes}
                      edgeTypes={edgeTypes}
                      onNodeDoubleClick={(_, n) =>
                        setSelection({ type: 'node', id: n.id })
                      }
                      onEdgeClick={(_, e) =>
                        setSelection({ type: 'edge', id: e.id })
                      }
                      fitView
                      fitViewOptions={{ padding: 0.25 }}
                      minZoom={0.3}
                      maxZoom={1.5}
                      deleteKeyCode={null}
                    >
                      <Background gap={20} size={1} color="#d5dfd9" />
                      <Controls showInteractive={false} />
                      <MiniMap
                        nodeColor={(n) =>
                          catalog.find((c) => c.kind === n.data.kind)?.color ??
                          '#777'
                        }
                        pannable
                        zoomable
                      />
                    </ReactFlow>
                    <div className="canvas-label">
                      <span className="green-dot" />
                      {environment.toLowerCase()} / commerce infrastructure
                    </div>
                    <span className="canvas-version">DESIGN MODE</span>
                    <div className="canvas-legend">
                      <span>
                        <i style={{ background: '#b77b33' }} />
                        Application
                      </span>
                      <span>
                        <i style={{ background: '#397b60' }} />
                        Infrastructure
                      </span>
                      <span>
                        <i style={{ background: '#6577b3' }} />
                        Dependency
                      </span>
                    </div>
                  </div>
                  <Tabs
                    className="result-tabs"
                    value={tab}
                    onValueChange={(v) => setTab(String(v))}
                  >
                    <TabsList variant="line">
                      <TabsTrigger value="issues">
                        <ShieldCheck size={15} /> 검증 결과{' '}
                        <span
                          className={
                            errors.length ? 'issue-count' : 'good-count'
                          }
                        >
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
                                if (issue.nodeId) {
                                  setSelection({
                                    type: 'node',
                                    id: issue.nodeId,
                                  });
                                  void flow.fitView({
                                    nodes: [{ id: issue.nodeId }],
                                    duration: 250,
                                    padding: 0.8,
                                  });
                                }
                              }}
                            >
                              <AlertTriangle
                                size={15}
                                color={
                                  issue.severity === 'error'
                                    ? '#c45151'
                                    : '#ac862f'
                                }
                              />
                              <span>{issue.message}</span>
                              <small>
                                {
                                  nodes.find((n) => n.id === issue.nodeId)?.data
                                    .label
                                }
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
                                {persistence.validation
                                  ? '서버 설계 검증을 통과했습니다.'
                                  : '로컬 기본 검사 결과입니다. 서버 검증을 실행하세요.'}
                              </strong>
                              <p>
                                연결 규칙과 VM 포트 충돌 검사가 완료되었습니다.
                                실제 통신 가능 여부는 별도 확인이 필요합니다.
                              </p>
                            </div>
                          </div>
                        )}
                      </div>
                    </TabsContent>
                    <TabsContent value="blueprint">
                      <pre className="blueprint-code">
                        {files['topology.json']}
                      </pre>
                    </TabsContent>
                  </Tabs>
                </section>
              </div>
              <footer>
                <output>
                  {message || '변경 후 설계 저장 버튼을 누르세요.'}
                </output>
                <span>
                  VM 초안 지원 <span className="footer-dot">·</span> Helm /
                  Terraform 예정
                </span>
              </footer>
            </>
          )}
        </main>
        <AlertDialog open={confirmReload} onOpenChange={setConfirmReload}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                서버 버전으로 다시 불러올까요?
              </AlertDialogTitle>
              <AlertDialogDescription>
                현재 미저장 편집은 사라집니다. 필요하면 취소 후 JSON을
                다운로드하세요.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>취소</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => {
                  setConfirmReload(false);
                  persistence.reload();
                }}
              >
                다시 불러오기
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
        <TopologyImportDialog
          open={importOpen}
          dirty={persistence.dirty}
          environment={persistence.environment}
          onOpenChange={setImportOpen}
          onImport={importTopology}
        />
        <Inspector
          artifacts={artifacts.items}
          errors={
            persistence.validation?.issues.filter((issue) =>
              issue.nodeIds.includes(selectedNode?.id ?? ''),
            ) ?? []
          }
          nodes={nodes}
          place={place}
          node={persistence.busy ? undefined : selectedNode}
          edge={persistence.busy ? undefined : selectedEdge}
          onClose={() => setSelection(null)}
          updateNode={(data) => {
            setNodes((ns) =>
              ns.map((n) =>
                n.id === selection?.id
                  ? { ...n, data: { ...n.data, ...data } }
                  : n,
              ),
            );
            if (typeof data.port === 'number') {
              setEdges((es) =>
                es.map((e) =>
                  e.target === selection?.id && e.data?.port !== undefined
                    ? {
                        ...e,
                        data: { ...e.data, port: data.port! },
                        label: 'TCP · ' + data.port,
                      }
                    : e,
                ),
              );
            }
          }}
          remove={remove}
        />
      </div>
    </SidebarProvider>
  );
}
