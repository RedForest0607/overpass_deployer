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
  useReactFlow,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  Boxes,
  Package,
  ArrowUpRight,
  ShieldCheck,
  ChevronRight,
  Moon,
  Sun,
} from 'lucide-react';
import {
  SidebarProvider,
  Sidebar,
  SidebarContent,
} from '@/components/ui/sidebar';
import {
  catalog,
  validate,
  type Kind,
  type TopologyNode,
  type TopologyEdge,
} from '../topology/model';
import ResourceNode from '../topology/ResourceNode';
import TargetNode from '../topology/TargetNode';
import CommunicationEdge from '../topology/CommunicationEdge';
import { isTarget } from '../topology/placement';
import TopologyPalette from './TopologyPalette';
import TopologyResults from './TopologyResults';
import TopologyToolbar from './TopologyToolbar';
import Inspector from '../topology/Inspector';
import Choice from '../topology/Choice';
import { downloadBlob } from '../../lib/download';
import ServerPackage from '../generation/ServerPackage';
import ArtifactManager from '../artifacts/ArtifactManager';
import { useArtifacts } from '../artifacts/useArtifacts';
import { applyArtifactMappings } from '../artifacts/artifact-mapping';
import type { ArtifactAssignments } from '../artifacts/artifact-mapping';

import { usePersistence } from './usePersistence';
import { toAPI } from '../topology/api-adapter';
import { defaultDeploymentProfile } from '../topology/api-adapter';
import type { DeploymentProfile } from '../../lib/api/contracts';
import TopologyImportDialog from '../topology/TopologyImportDialog';
import type { ImportedTopology } from '../topology/topology-import';
import ProjectsPanel from './ProjectsPanel';
import WorkspaceNavigation from './WorkspaceNavigation';
import { useTheme } from './useTheme';
import UnsavedChangesDialog from './UnsavedChangesDialog';
import { useTopologyNavigation } from './useTopologyNavigation';
import { useTopologyEditor } from './useTopologyEditor';
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
  const { theme, toggleTheme } = useTheme();
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
  const { place, valid, connect, add, remove, updateNode } = useTopologyEditor({
    nodes,
    edges,
    selection,
    setNodes,
    setEdges,
    setSelection,
    setMessage,
    setLastRemoval,
    setPaletteOpen,
  });
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
  const {
    pendingNavigation,
    navigationAttempted,
    requestTopology,
    cancelNavigation,
    saveAndNavigate,
    discardAndNavigate,
  } = useTopologyNavigation(persistence, () => setView('topology'));
  const project = persistence.active?.name ?? '프로젝트를 선택하세요';
  const environment = persistence.environment.toUpperCase();
  const saveState = persistence.conflict
    ? { tone: 'error', label: '저장 충돌' }
    : persistence.busy
      ? { tone: 'busy', label: '처리 중' }
      : persistence.dirty
        ? { tone: 'warning', label: '미저장 변경' }
        : { tone: 'success', label: `저장됨 · r${persistence.revision}` };
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
  const topologyDocument = useMemo(
    () => toAPI(nodes, edges, viewport, deployment),
    [nodes, edges, viewport, deployment],
  );
  const topologyJSON = useMemo(
    () =>
      JSON.stringify(
        { environment: persistence.environment, ...topologyDocument },
        null,
        2,
      ),
    [persistence.environment, topologyDocument],
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
                data: { ...edge.data, layout: normalized },
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
          ...edge.data,
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
  const selectedNode =
    selection?.type === 'node'
      ? nodes.find((n) => n.id === selection.id)
      : undefined;
  const selectedEdge =
    selection?.type === 'edge'
      ? edges.find((e) => e.id === selection.id)
      : undefined;

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
            <WorkspaceNavigation
              view={view}
              project={persistence.active?.name}
              environment={environment}
              onNavigate={(next) =>
                next === 'packages' ? preview() : setView(next)
              }
            />
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
              Workspace <ChevronRight size={13} />{' '}
              <b>{view === 'projects' ? '프로젝트' : project}</b>{' '}
              <ChevronRight size={13} />{' '}
              {view === 'topology'
                ? 'Topology'
                : view === 'projects'
                  ? 'Projects'
                  : view === 'artifacts'
                    ? 'Artifacts'
                    : 'Delivery package'}
            </span>
            <button
              className="theme-toggle"
              onClick={toggleTheme}
              aria-label={
                theme === 'dark' ? '라이트 모드로 전환' : '다크 모드로 전환'
              }
              aria-pressed={theme === 'dark'}
            >
              {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
              <span>{theme === 'dark' ? '라이트 모드' : '다크 모드'}</span>
            </button>
            <span className="avatar" aria-label="Overpass workspace">
              OP
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
                {persistence.active && (
                  <span className="workspace-context">
                    <span className="pill">{environment}</span>
                    <output
                      className={`save-state save-state--${saveState.tone}`}
                    >
                      {saveState.label}
                    </output>
                  </span>
                )}
              </h1>
              <p>
                {view === 'projects'
                  ? '프로젝트와 환경별 배포 구성을 저장하고 관리하세요.'
                  : view === 'artifacts'
                    ? `${project}에 등록된 JAR, 설정, 배포 파일을 관리하세요.`
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
                    requestTopology(
                      persistence.active,
                      value.toLowerCase() as 'dev' | 'qa' | 'prod',
                    );
                }}
              />
              {view === 'topology' && (
                <>
                  <button
                    className={persistence.dirty ? 'primary' : undefined}
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
                  <button
                    className={persistence.dirty ? undefined : 'primary'}
                    onClick={preview}
                  >
                    <Package size={16} /> 산출물 미리보기{' '}
                    <ArrowUpRight size={16} />
                  </button>
                </>
              )}
            </div>
          </section>
          <output
            className={`workspace-notice${persistence.conflict ? ' workspace-notice--error' : ''}`}
          >
            {persistence.notice}
          </output>
          {view === 'projects' ? (
            <ProjectsPanel
              projects={persistence.projects}
              busy={persistence.busy}
              open={(project) =>
                requestTopology(project, persistence.environment)
              }
              edit={persistence.editProject}
              refresh={() => {
                void persistence.refresh();
              }}
            />
          ) : view === 'artifacts' ? (
            <ArtifactManager
              key={persistence.active?.id ?? 'none'}
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
              <TopologyToolbar
                nodeCount={nodes.length}
                connectionCount={visibleEdges.length}
                paletteOpen={paletteOpen}
                setPaletteOpen={setPaletteOpen}
                canUndo={!!lastRemoval}
                undo={() => {
                  if (!lastRemoval) return;
                  setNodes(lastRemoval.nodes);
                  setEdges(lastRemoval.edges);
                  setLastRemoval(null);
                  setMessage('삭제를 취소했습니다.');
                }}
                canImport={!persistence.busy && !!persistence.active}
                openImport={() => setImportOpen(true)}
                downloadJSON={() =>
                  downloadBlob(
                    new Blob([topologyJSON], { type: 'application/json' }),
                    project + '-topology.json',
                  )
                }
              />
              <div
                className="editor"
                inert={persistence.busy || !persistence.active}
              >
                <TopologyPalette
                  paletteOpen={paletteOpen}
                  deployment={deployment}
                  setDeployment={setDeployment}
                  add={add}
                />
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
                      colorMode={theme}
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
                      defaultViewport={viewport}
                      minZoom={0.3}
                      maxZoom={1.5}
                      deleteKeyCode={null}
                    >
                      <Background
                        gap={20}
                        size={1}
                        color={theme === 'dark' ? '#334b43' : '#d5dfd9'}
                      />
                      <Controls showInteractive={false} />
                      <MiniMap
                        bgColor={theme === 'dark' ? '#15221f' : '#fff'}
                        maskColor={theme === 'dark' ? '#0b151ab3' : '#f0f2f3b3'}
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
                  <TopologyResults
                    tab={tab}
                    setTab={setTab}
                    issues={issues}
                    nodes={nodes}
                    serverValidated={!!persistence.validation}
                    topologyJSON={topologyJSON}
                    onSelectNode={(nodeId) => {
                      setSelection({ type: 'node', id: nodeId });
                      void flow.fitView({
                        nodes: [{ id: nodeId }],
                        duration: 250,
                        padding: 0.8,
                      });
                    }}
                  />
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
        <UnsavedChangesDialog
          open={pendingNavigation !== null}
          busy={persistence.busy}
          currentLabel={`${persistence.active?.name ?? '현재 프로젝트'} / ${environment}`}
          targetLabel={`${pendingNavigation?.project.name ?? ''} / ${pendingNavigation?.environment.toUpperCase() ?? ''}`}
          status={
            navigationAttempted && !persistence.busy ? persistence.notice : ''
          }
          onCancel={cancelNavigation}
          onDiscard={() => void discardAndNavigate()}
          onSave={() => void saveAndNavigate()}
        />
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
          updateNode={updateNode}
          remove={remove}
        />
      </div>
    </SidebarProvider>
  );
}
