import { parseAllDocuments } from 'yaml';
import type { Artifact } from '../../lib/api/delivery.ts';

export function isEditableArtifact(artifact: Artifact) {
  if (artifact.kind === 'jar' || artifact.kind === 'archive') return false;
  const name = artifact.originalName.toLowerCase();
  return (
    name.endsWith('.yml') || name.endsWith('.yaml') || name.endsWith('.xml')
  );
}

export function validateArtifactText(name: string, value: string) {
  if (!value.trim()) return '파일 내용을 입력하세요.';
  const lower = name.toLowerCase();
  if (lower.endsWith('.yml') || lower.endsWith('.yaml')) {
    const documents = parseAllDocuments(value, { prettyErrors: true });
    return documents.flatMap((document) => document.errors)[0]?.message;
  }
  if (lower.endsWith('.xml') && typeof DOMParser !== 'undefined') {
    const document = new DOMParser().parseFromString(value, 'application/xml');
    const error = document.querySelector('parsererror');
    if (error) return 'XML 문법을 확인하세요: ' + error.textContent?.trim();
  }
  return undefined;
}

export function artifactMediaType(name: string) {
  return name.toLowerCase().endsWith('.xml')
    ? 'application/xml'
    : 'application/yaml';
}
