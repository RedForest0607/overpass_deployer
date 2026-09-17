// Code generated from schemas/openapi-v1.yaml by scripts/generate-api-contracts.mjs. DO NOT EDIT.
import type { Blueprint, Layout } from './contracts.ts';
export type { Blueprint, Layout };

export type Environment = "dev" | "qa" | "prod";

export type ArtifactKind = "jar" | "config-template" | "env-example" | "file" | "archive" | "script-values";

export type Output = "draft" | "package";

export type Project = { "id": string; "name": string; "description": string; "revision": number; "createdAt": string; "updatedAt": string; };

export type CreateProject = { "name": string; "description": string; };

export type UpdateProject = { "expectedRevision": number; "name"?: string; "description"?: string; };

export type Artifact = { "id": string; "projectId": string; "kind": ArtifactKind; "originalName": string; "sizeBytes": number; "sha256": string; "createdAt": string; };

export type ArtifactContent = { "id": string; "content": string; "encoding": "utf-8"; };

export type Position = { "x": number; "y": number; };

export type SaveTopology = { "expectedRevision": number; "blueprint": Blueprint; "layout": Layout; };

export type Topology = { "projectId": string; "environment": Environment; "revision": number; "updatedAt": string; "blueprint": Blueprint; "layout": Layout; };

export type ValidationRequest = { "environment": Environment; "clientRequestId": string; "blueprint": Blueprint; "generator"?: string; "purpose": "design" | "package"; };

export type Issue = { "code": string; "severity": "warning" | "error"; "stage": "model" | "deployment" | "artifact" | "generator"; "nodeIds": Array<string>; "connectionIds": Array<string>; "path": string; "message": string; "suggestion"?: string; };

export type ValidationResult = { "clientRequestId": string; "valid": boolean; "issues": Array<Issue>; };

export type CreateGeneration = { "environment": Environment; "revision": number; "generator": "crosswalk-vm"; "output": Output; };

export type Generation = { "id": string; "projectId": string; "environment": Environment; "revision": number; "generator": string; "output": Output; "status": "queued" | "running" | "succeeded" | "failed"; "attempts": number; "packageId": string | null; "retryOfGenerationId": string | null; "error": GenerationFailure | null; "createdAt": string; "updatedAt": string; };

export type GenerationFailure = { "code": string; "message": string; };

export type PackageFile = { "id": string; "path": string; "mediaType": string; "sizeBytes": number; "sha256": string; "previewable": boolean; };

export type Package = { "id": string; "projectId": string; "environment": Environment; "revision": number; "output": Output; "generator": string; "generatorVersion": string; "chartFormat": "crosswalk/v1"; "createdAt": string; "files": Array<PackageFile>; };

export type FilePreview = { "id": string; "content": string; "encoding": "utf-8"; "truncated": boolean; };

export type GeneratorCapability = { "id": string; "outputs": Array<Output>; "chartFormats": Array<"crosswalk/v1">; "supportedKinds": Array<string>; "supportedModes": Array<string>; };

export type Limits = { "jarBytes": number; "textArtifactBytes": number; "previewBytes": number; "topologyNodes": number; "topologyConnections": number; };

export type Capabilities = { "blueprintVersions": Array<string>; "generators": Array<GeneratorCapability>; "limits": Limits; };

export type ErrorBody = { "code": string; "message": string; "requestId": string; "details"?: Record<string, unknown>; };

export type ErrorEnvelope = { "error": ErrorBody; };
