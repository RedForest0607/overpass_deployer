import { request } from './client.ts';
import type {
  Blueprint,
  CreateProject,
  Environment,
  Issue,
  Layout,
  Project,
  SaveTopology,
  Topology,
  UpdateProject,
  ValidationRequest,
  ValidationResult,
} from './generated.ts';

export type { Project, Topology };
export type ServerIssue = Issue;
export type Validation = ValidationResult;
const projectPath = (id: string) => '/projects/' + encodeURIComponent(id);

export const projectsAPI = {
  async list(signal?: AbortSignal): Promise<Project[]> {
    const items: Project[] = [];
    let cursor: string | null = null;
    do {
      const page: { items: Project[]; nextCursor: string | null } =
        await request(
          '/projects?limit=100' +
            (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
          { signal },
        );
      items.push(...page.items);
      cursor = page.nextCursor;
    } while (cursor);
    return items;
  },
  create(name: string, description: string) {
    const body: CreateProject = { name, description };
    return request<Project>('/projects', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },
  update(project: Project, name: string, description: string) {
    const body: UpdateProject = {
      expectedRevision: project.revision,
      name,
      description,
    };
    return request<Project>(projectPath(project.id), {
      method: 'PATCH',
      body: JSON.stringify(body),
    });
  },
  load(id: string, environment: Environment, signal?: AbortSignal) {
    return request<Topology>(projectPath(id) + '/topologies/' + environment, {
      signal,
    });
  },
  save(
    id: string,
    environment: Environment,
    expectedRevision: number,
    document: { blueprint: Blueprint; layout: Layout },
  ) {
    const body: SaveTopology = { expectedRevision, ...document };
    return request<Topology>(projectPath(id) + '/topologies/' + environment, {
      method: 'PUT',
      body: JSON.stringify(body),
    });
  },
  validate(
    id: string,
    environment: Environment,
    blueprint: Blueprint,
    clientRequestId: string,
  ) {
    const body: ValidationRequest = {
      environment,
      blueprint,
      clientRequestId,
      purpose: 'design',
    };
    return request<Validation>(projectPath(id) + '/validations', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },
};
