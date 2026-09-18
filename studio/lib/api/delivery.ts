import { request, requestBlob } from './client.ts';
import type {
  Artifact,
  ArtifactContent,
  ArtifactKind,
  Blueprint,
  Capabilities,
  CreateGeneration,
  Environment,
  Generation,
  GeneratorCapability,
  Package as GeneratedPackage,
  PackageFile,
  ValidationRequest,
} from './generated.ts';
import type { Validation } from './projects.ts';
import type { GeneratorID } from './contracts.ts';

export type {
  Artifact,
  ArtifactContent,
  ArtifactKind,
  Capabilities,
  Generation,
  GeneratorCapability,
  GeneratedPackage,
  PackageFile,
};

const projectPath = (id: string) => '/projects/' + encodeURIComponent(id);

export const deliveryAPI = {
  capabilities(signal?: AbortSignal) {
    return request<Capabilities>('/capabilities', { signal });
  },
  async listArtifacts(projectId: string, signal?: AbortSignal) {
    const items: Artifact[] = [];
    let cursor: string | null = null;
    do {
      const page: { items: Artifact[]; nextCursor: string | null } =
        await request(
          projectPath(projectId) +
            '/artifacts?limit=100' +
            (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
          { signal },
        );
      items.push(...page.items);
      cursor = page.nextCursor;
    } while (cursor);
    return items;
  },
  uploadArtifact(
    projectId: string,
    kind: ArtifactKind,
    file: File,
    displayName?: string,
  ) {
    const form = new FormData();
    form.set('kind', kind);
    form.set('file', file);
    if (displayName?.trim()) form.set('displayName', displayName.trim());
    return request<Artifact>(projectPath(projectId) + '/artifacts', {
      method: 'POST',
      body: form,
    });
  },
  getArtifactContent(
    projectId: string,
    artifactId: string,
    signal?: AbortSignal,
  ) {
    return request<ArtifactContent>(
      projectPath(projectId) +
        '/artifacts/' +
        encodeURIComponent(artifactId) +
        '/content',
      { signal },
    );
  },
  validatePackage(
    projectId: string,
    environment: Environment,
    blueprint: Blueprint,
    clientRequestId: string,
    generator: GeneratorID = 'crosswalk-vm',
  ) {
    const body: ValidationRequest = {
      environment,
      blueprint,
      clientRequestId,
      purpose: 'package',
      generator,
    };
    return request<Validation>(projectPath(projectId) + '/validations', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },
  createGeneration(
    projectId: string,
    environment: Environment,
    revision: number,
    output: 'draft' | 'package',
    idempotencyKey: string,
    generator: GeneratorID = 'crosswalk-vm',
  ) {
    const body: CreateGeneration = {
      environment,
      revision,
      generator,
      output,
    };
    return request<Generation>(projectPath(projectId) + '/generations', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(body),
    });
  },
  getGeneration(projectId: string, generationId: string, signal?: AbortSignal) {
    return request<Generation>(
      projectPath(projectId) +
        '/generations/' +
        encodeURIComponent(generationId),
      { signal },
    );
  },
  listGenerations(
    projectId: string,
    environment?: Environment,
    cursor?: string,
    signal?: AbortSignal,
  ) {
    const query = new URLSearchParams({ limit: '10' });
    if (environment) query.set('environment', environment);
    if (cursor) query.set('cursor', cursor);
    return request<{ items: Generation[]; nextCursor: string | null }>(
      projectPath(projectId) + '/generations?' + query.toString(),
      { signal },
    );
  },
  retryGeneration(
    projectId: string,
    generationId: string,
    idempotencyKey: string,
  ) {
    return request<Generation>(
      projectPath(projectId) +
        '/generations/' +
        encodeURIComponent(generationId) +
        '/retry',
      {
        method: 'POST',
        headers: { 'Idempotency-Key': idempotencyKey },
      },
    );
  },
  getPackage(projectId: string, packageId: string, signal?: AbortSignal) {
    return request<GeneratedPackage>(
      projectPath(projectId) + '/packages/' + encodeURIComponent(packageId),
      { signal },
    );
  },
  previewFile(
    projectId: string,
    packageId: string,
    fileId: string,
    signal?: AbortSignal,
  ) {
    return request<{
      id: string;
      content: string;
      encoding: 'utf-8';
      truncated: boolean;
    }>(
      projectPath(projectId) +
        '/packages/' +
        encodeURIComponent(packageId) +
        '/files/' +
        encodeURIComponent(fileId),
      { signal },
    );
  },
  downloadPackage(projectId: string, packageId: string) {
    return requestBlob(
      projectPath(projectId) +
        '/packages/' +
        encodeURIComponent(packageId) +
        '/download',
    );
  },
};
