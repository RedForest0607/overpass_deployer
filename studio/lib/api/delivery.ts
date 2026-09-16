import { request, requestBlob } from './client.ts';
import type { Blueprint, Environment } from './contracts.ts';
import type { Validation } from './projects.ts';

export type ArtifactKind =
  | 'jar'
  | 'config-template'
  | 'env-example'
  | 'file'
  | 'archive'
  | 'script-values';

export type Artifact = {
  id: string;
  projectId: string;
  kind: ArtifactKind;
  originalName: string;
  sizeBytes: number;
  sha256: string;
  createdAt: string;
};

export type ArtifactContent = {
  id: string;
  content: string;
  encoding: 'utf-8';
};

export type GeneratorCapability = {
  id: string;
  outputs: ('draft' | 'package')[];
  supportedKinds: string[];
  supportedModes: string[];
};

export type Capabilities = {
  blueprintVersions: string[];
  generators: GeneratorCapability[];
  limits: {
    jarBytes: number;
    textArtifactBytes: number;
    previewBytes: number;
    topologyNodes: number;
    topologyConnections: number;
  };
};

export type Generation = {
  id: string;
  projectId: string;
  environment: Environment;
  revision: number;
  status: 'queued' | 'running' | 'succeeded' | 'failed';
  generator: string;
  output: 'draft' | 'package';
  attempts: number;
  packageId: string | null;
  error: { code: string; message: string } | null;
  createdAt: string;
  updatedAt: string;
};

export type PackageFile = {
  id: string;
  path: string;
  mediaType: string;
  sizeBytes: number;
  sha256: string;
  previewable: boolean;
};

export type GeneratedPackage = {
  id: string;
  projectId: string;
  environment: Environment;
  revision: number;
  output: 'draft' | 'package';
  generator: string;
  generatorVersion: string;
  chartFormat: string;
  createdAt: string;
  files: PackageFile[];
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
  ) {
    return request<Validation>(projectPath(projectId) + '/validations', {
      method: 'POST',
      body: JSON.stringify({
        environment,
        blueprint,
        clientRequestId,
        purpose: 'package',
        generator: 'crosswalk-vm',
      }),
    });
  },
  createGeneration(
    projectId: string,
    environment: Environment,
    revision: number,
    output: 'draft' | 'package',
    idempotencyKey: string,
  ) {
    return request<Generation>(projectPath(projectId) + '/generations', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify({
        environment,
        revision,
        generator: 'crosswalk-vm',
        output,
      }),
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
