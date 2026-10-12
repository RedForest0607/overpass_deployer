'use client';
import {
  useEffect,
  useEffectEvent,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import { APIError } from '../../lib/api/client';
import {
  projectsAPI,
  type Project,
  type Validation,
} from '../../lib/api/projects';
import type { Environment } from '../../lib/api/contracts';
import type { DeploymentProfile } from '../../lib/api/contracts';
import {
  defaultDeploymentProfile,
  fromAPI,
  toAPI,
} from '../topology/api-adapter';
import type { TopologyNode, TopologyEdge } from '../topology/model';
import { isCurrentValidation } from './validation-state';

type Graph = {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
  viewport?: { x: number; y: number; zoom: number };
  deployment?: DeploymentProfile;
};
type OpenOptions = { discardChanges?: boolean };
const viewport = { x: 0, y: 0, zoom: 1 };
export const graphKey = (graph: Graph) =>
  JSON.stringify(
    toAPI(
      graph.nodes,
      graph.edges,
      graph.viewport ?? viewport,
      graph.deployment,
    ),
  );

export function usePersistence(graph: Graph, replace: (graph: Graph) => void) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [active, setActive] = useState<Project | null>(null);
  const [environment, setEnvironment] = useState<Environment>('dev');
  const [revision, setRevision] = useState(0);
  const [baseline, setBaseline] = useState(() => graphKey(graph));
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');
  const [conflict, setConflict] = useState(false);
  const [validation, setValidation] = useState<{
    key: string;
    value: Validation;
  } | null>(null);
  const currentKey = graphKey(graph);
  const current = useRef({ graph, key: currentKey });
  useLayoutEffect(() => {
    current.current = { graph, key: currentKey };
  }, [graph, currentKey]);
  const operation = useRef(0);
  const alive = useRef(true);
  const lock = useRef(false);
  const dirty = currentKey !== baseline;

  function report(error: unknown) {
    if (error instanceof APIError && error.status === 409) {
      setNotice(
        '저장 충돌: 편집 내용은 유지했습니다. JSON으로 보관한 뒤 서버 버전을 다시 불러오세요.',
      );
    } else
      setNotice(
        error instanceof Error ? error.message : '요청에 실패했습니다.',
      );
  }

  async function load(project: Project, env: Environment): Promise<boolean> {
    const token = ++operation.current;
    lock.current = true;
    setBusy(true);
    try {
      let restored: Graph = {
        nodes: [],
        edges: [],
        deployment: defaultDeploymentProfile,
      };
      let nextRevision = 0;
      try {
        const value = await projectsAPI.load(project.id, env);
        restored = fromAPI(value.blueprint, value.layout);
        nextRevision = value.revision;
      } catch (error) {
        if (!(error instanceof APIError && error.code === 'TOPOLOGY_NOT_FOUND'))
          throw error;
      }
      if (!alive.current || token !== operation.current) return false;
      replace(restored);
      setBaseline(graphKey(restored));
      setRevision(nextRevision);
      setActive(project);
      setEnvironment(env);
      setConflict(false);
      setValidation(null);
      setNotice(
        nextRevision
          ? '서버 설계를 불러왔습니다.'
          : '이 환경의 첫 설계입니다. 노드를 추가하고 저장하세요.',
      );
      const url = new URL(window.location.href);
      url.searchParams.set('project', project.id);
      url.searchParams.set('environment', env);
      window.history.replaceState(null, '', url);
      return true;
    } catch (error) {
      if (alive.current && token === operation.current) report(error);
      return false;
    } finally {
      if (alive.current && token === operation.current) {
        lock.current = false;
        setBusy(false);
      }
    }
  }

  async function refresh() {
    if (lock.current) return;
    const token = ++operation.current;
    lock.current = true;
    setBusy(true);
    try {
      const values = await projectsAPI.list();
      if (!alive.current || token !== operation.current) return;
      setProjects(values);
      if (!active && values.length) {
        const query = new URLSearchParams(window.location.search);
        const env = query.get('environment');
        await load(
          values.find((p) => p.id === query.get('project')) ?? values[0],
          env === 'qa' || env === 'prod' ? env : 'dev',
        );
      } else
        setNotice(
          values.length
            ? '프로젝트 목록을 갱신했습니다.'
            : '프로젝트를 생성해 시작하세요.',
        );
    } catch (error) {
      if (alive.current && token === operation.current) report(error);
    } finally {
      if (alive.current && token === operation.current) {
        lock.current = false;
        setBusy(false);
      }
    }
  }
  const initialRefresh = useEffectEvent(refresh);
  useEffect(() => {
    const operationRef = operation;
    const lockRef = lock;
    const aliveRef = alive;
    alive.current = true;
    let disposed = false;
    void Promise.resolve().then(() => {
      if (!disposed) return initialRefresh();
    });
    return () => {
      disposed = true;
      aliveRef.current = false;
      operationRef.current++;
      lockRef.current = false;
    };
  }, []);
  useEffect(() => {
    const warn = (event: BeforeUnloadEvent) => {
      if (dirty) {
        event.preventDefault();
      }
    };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [dirty]);

  async function open(
    project: Project,
    env = environment,
    options: OpenOptions = {},
  ): Promise<boolean> {
    if (lock.current) return false;
    if (dirty && !options.discardChanges) {
      setNotice(
        '미저장 변경이 있습니다. 먼저 저장하거나 서버 버전 다시 불러오기로 편집을 취소하세요.',
      );
      return false;
    }
    return load(project, env);
  }
  async function save(): Promise<boolean> {
    if (!active || lock.current || conflict) return false;
    lock.current = true;
    setBusy(true);
    setValidation(null);
    const snapshot = current.current;
    try {
      const result = await projectsAPI.save(
        active.id,
        environment,
        revision,
        toAPI(
          snapshot.graph.nodes,
          snapshot.graph.edges,
          snapshot.graph.viewport ?? viewport,
          snapshot.graph.deployment,
        ),
      );
      if (!alive.current) return false;
      setRevision(result.revision);
      setBaseline(snapshot.key);
      if (current.current.key !== snapshot.key) {
        setNotice(
          '저장 중 설계가 다시 변경되었습니다. 최신 변경을 저장한 뒤 이동하세요.',
        );
        return false;
      }
      setNotice('설계를 저장했습니다. revision ' + result.revision);
      return true;
    } catch (error) {
      if (alive.current) {
        if (error instanceof APIError && error.status === 409)
          setConflict(true);
        report(error);
      }
      return false;
    } finally {
      if (alive.current) {
        lock.current = false;
        setBusy(false);
      }
    }
  }
  async function validate() {
    if (!active || lock.current) return;
    lock.current = true;
    setBusy(true);
    const snapshot = current.current;
    const clientRequestId = crypto.randomUUID();
    try {
      const value = await projectsAPI.validate(
        active.id,
        environment,
        toAPI(
          snapshot.graph.nodes,
          snapshot.graph.edges,
          viewport,
          snapshot.graph.deployment,
        ).blueprint,
        clientRequestId,
      );
      if (!alive.current) return;
      if (
        !isCurrentValidation(
          snapshot.key,
          current.current.key,
          clientRequestId,
          value.clientRequestId,
        )
      ) {
        setNotice(
          '검사 중 설계가 변경되어 결과를 폐기했습니다. 다시 검증하세요.',
        );
        return;
      }
      setValidation({ key: snapshot.key, value });
      setNotice(
        value.valid
          ? '서버 설계 검증 통과 · 실제 배포 검증은 아닙니다.'
          : '서버 검증 오류를 확인하세요.',
      );
    } catch (error) {
      if (alive.current) report(error);
    } finally {
      if (alive.current) {
        lock.current = false;
        setBusy(false);
      }
    }
  }
  async function editProject(
    name: string,
    description: string,
    project?: Project,
  ) {
    if (lock.current) return false;
    lock.current = true;
    setBusy(true);
    try {
      const value = project
        ? await projectsAPI.update(project, name, description)
        : await projectsAPI.create(name, description);
      if (!alive.current) return false;
      setProjects((values) =>
        project
          ? values.map((p) => (p.id === value.id ? value : p))
          : [...values, value],
      );
      if (active?.id === value.id) setActive(value);
      if (!active) await load(value, 'dev');
      setNotice(
        project ? '프로젝트를 수정했습니다.' : '프로젝트를 생성했습니다.',
      );
      return true;
    } catch (error) {
      if (alive.current) report(error);
      return false;
    } finally {
      if (alive.current) {
        lock.current = false;
        setBusy(false);
      }
    }
  }
  return {
    projects,
    active,
    environment,
    revision,
    busy,
    notice:
      validation && validation.key !== currentKey
        ? '설계가 변경되었습니다. 서버 검증을 다시 실행하세요.'
        : notice,
    dirty,
    conflict,
    validation: validation?.key === currentKey ? validation.value : null,
    open,
    save,
    validate,
    refresh,
    editProject,
    reload: () => {
      if (active && !lock.current) void load(active, environment);
    },
  };
}
