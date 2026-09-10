import { request } from './client.ts';
import type { Blueprint, Layout, Environment } from './contracts.ts';

export type Project = {
  id: string;
  name: string;
  description: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
};
export type Topology = {
  projectId: string;
  environment: Environment;
  revision: number;
  blueprint: Blueprint;
  layout: Layout;
};
export type ServerIssue = {
  code: string;
  severity: 'error' | 'warning';
  stage: string;
  nodeIds: string[];
  connectionIds: string[];
  path: string;
  message: string;
};
export type Validation = {
  clientRequestId: string;
  valid: boolean;
  issues: ServerIssue[];
};
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
    return request<Project>('/projects', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    });
  },
  update(project: Project, name: string, description: string) {
    return request<Project>(projectPath(project.id), {
      method: 'PATCH',
      body: JSON.stringify({
        expectedRevision: project.revision,
        name,
        description,
      }),
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
    return request<Topology>(projectPath(id) + '/topologies/' + environment, {
      method: 'PUT',
      body: JSON.stringify({ expectedRevision, ...document }),
    });
  },
  validate(
    id: string,
    environment: Environment,
    blueprint: Blueprint,
    clientRequestId: string,
  ) {
    return request<Validation>(projectPath(id) + '/validations', {
      method: 'POST',
      body: JSON.stringify({
        environment,
        blueprint,
        clientRequestId,
        purpose: 'design',
      }),
    });
  },
};
