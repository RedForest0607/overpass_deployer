'use client';

import { useRef, useState, type SyntheticEvent } from 'react';
import {
  Check,
  Clipboard,
  FileArchive,
  FilePenLine,
  RefreshCw,
  Upload,
} from 'lucide-react';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import Choice from '../topology/Choice';
import ArtifactEditorDialog from './ArtifactEditorDialog';
import ArtifactMappingPanel from './ArtifactMappingPanel';
import {
  analyzeArtifactMappings,
  type ArtifactAssignments,
} from './artifact-mapping';
import { artifactMediaType, isEditableArtifact } from './artifact-text';
import type { TopologyNode } from '../topology/model';
import type {
  Artifact,
  ArtifactContent,
  ArtifactKind,
} from '../../lib/api/delivery';

const kinds: { value: ArtifactKind; label: string }[] = [
  { value: 'jar', label: 'Java JAR' },
  { value: 'config-template', label: '설정 템플릿' },
  { value: 'env-example', label: '환경 변수 예제' },
  { value: 'file', label: '일반 파일' },
  { value: 'archive', label: 'ZIP / TAR / TGZ' },
  { value: 'script-values', label: '스크립트 values' },
];

function formatBytes(value: number) {
  if (value < 1024) return value + ' B';
  if (value < 1024 * 1024) return (value / 1024).toFixed(1) + ' KB';
  return (value / 1024 / 1024).toFixed(1) + ' MB';
}

export default function ArtifactManager({
  projectId,
  projectName,
  artifacts,
  busy,
  error,
  upload,
  refresh,
  readText,
  nodes,
  mapArtifacts,
}: {
  projectId?: string;
  projectName: string;
  artifacts: Artifact[];
  busy: boolean;
  error: string;
  upload: (
    kind: ArtifactKind,
    file: File,
    displayName?: string,
  ) => Promise<Artifact | null>;
  refresh: () => void;
  readText: (
    artifactId: string,
    signal?: AbortSignal,
  ) => Promise<ArtifactContent>;
  nodes: TopologyNode[];
  mapArtifacts: (
    assignments: ArtifactAssignments,
    availableArtifacts?: Artifact[],
  ) => number;
}) {
  const [kind, setKind] = useState<ArtifactKind>('jar');
  const [displayName, setDisplayName] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [message, setMessage] = useState('');
  const [editor, setEditor] = useState<{
    artifact: Artifact;
    content: string;
  } | null>(null);
  const [editorLoading, setEditorLoading] = useState('');
  const input = useRef<HTMLInputElement>(null);

  async function submit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage('');
    if (!file) {
      setMessage('업로드할 파일을 선택하세요.');
      return;
    }
    if (await upload(kind, file, displayName)) {
      setDisplayName('');
      setFile(null);
      if (input.current) input.current.value = '';
      setMessage('파일을 안전하게 등록했습니다.');
    }
  }

  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      setMessage('artifact ID를 복사했습니다.');
    } catch {
      setMessage('클립보드 권한이 없습니다. ID를 직접 선택해 복사하세요.');
    }
  }

  async function openEditor(artifact: Artifact) {
    setMessage('');
    setEditorLoading(artifact.id);
    try {
      const result = await readText(artifact.id);
      setEditor({ artifact, content: result.content });
    } catch (cause) {
      setMessage(
        cause instanceof Error
          ? cause.message
          : '파일 내용을 불러오지 못했습니다.',
      );
    } finally {
      setEditorLoading('');
    }
  }

  async function saveEditedArtifact(
    content: string,
    replaceReferences: boolean,
  ): Promise<string | void> {
    if (!editor) return '편집할 파일을 다시 선택하세요.';
    const { artifact } = editor;
    const file = new File([content], artifact.originalName, {
      type: artifactMediaType(artifact.originalName),
    });
    const created = await upload(artifact.kind, file, artifact.originalName);
    if (!created) return '새 artifact 버전을 저장하지 못했습니다.';

    let replaced = 0;
    if (replaceReferences) {
      const requirements = analyzeArtifactMappings(nodes, artifacts).filter(
        ({ currentRef }) => currentRef === artifact.id,
      );
      const assignments = Object.fromEntries(
        requirements.map(({ id }) => [id, created.id]),
      );
      replaced = mapArtifacts(assignments, [created, ...artifacts]);
    }
    setMessage(
      replaced
        ? `새 파일 버전을 등록하고 topology 참조 ${replaced}개를 교체했습니다. 설계를 저장하세요.`
        : '새 파일 버전을 등록했습니다.',
    );
  }

  return (
    <section className="artifact-view">
      <div className="artifact-layout">
        <form className="upload-card" onSubmit={submit}>
          <div className="section-heading">
            <span className="artifact-symbol">
              <Upload size={19} />
            </span>
            <div>
              <h2>파일 등록</h2>
              <p>
                파일은 SHA-256 기준으로 보관되며 현재 프로젝트 안에서만
                참조됩니다.
              </p>
            </div>
          </div>
          <div className="field">
            <span>파일 종류</span>
            <Choice
              label="파일 종류"
              value={kind}
              options={kinds}
              onChange={(value) => setKind(value as ArtifactKind)}
            />
          </div>
          <label className="field" htmlFor="artifact-file">
            <span>파일</span>
            <Input
              id="artifact-file"
              ref={input}
              type="file"
              disabled={!projectId || busy}
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
            />
          </label>
          <label className="field" htmlFor="artifact-display-name">
            <span>표시 이름 · 선택</span>
            <Input
              id="artifact-display-name"
              value={displayName}
              placeholder={file?.name || '원본 파일명을 사용합니다'}
              disabled={!projectId || busy}
              onChange={(event) => setDisplayName(event.target.value)}
            />
          </label>
          <button
            className="primary upload-submit"
            disabled={!projectId || !file || busy}
          >
            <Upload size={16} /> {busy ? '처리 중…' : '등록'}
          </button>
          {!projectId && (
            <p className="upload-error">프로젝트를 먼저 선택하세요.</p>
          )}
          {(error || message) && (
            <output className={error ? 'upload-error' : 'upload-success'}>
              {error || message}
            </output>
          )}
        </form>

        <div className="artifact-list">
          <div className="section-heading artifact-list-heading">
            <span className="artifact-symbol">
              <FileArchive size={19} />
            </span>
            <div>
              <h2>{projectName} 파일</h2>
              <p>{artifacts.length}개의 immutable artifact</p>
            </div>
            <button disabled={!projectId || busy} onClick={refresh}>
              <RefreshCw size={15} /> 새로고침
            </button>
          </div>
          <div className="table-shell">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>이름</TableHead>
                  <TableHead>종류</TableHead>
                  <TableHead>크기</TableHead>
                  <TableHead>SHA-256</TableHead>
                  <TableHead>등록일</TableHead>
                  <TableHead>작업</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {artifacts.map((artifact) => (
                  <TableRow key={artifact.id}>
                    <TableCell className="artifact-name">
                      {artifact.originalName}
                    </TableCell>
                    <TableCell>{artifact.kind}</TableCell>
                    <TableCell>{formatBytes(artifact.sizeBytes)}</TableCell>
                    <TableCell>
                      <code>{artifact.sha256.slice(0, 12)}…</code>
                    </TableCell>
                    <TableCell>
                      {new Date(artifact.createdAt).toLocaleString('ko-KR')}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1 whitespace-nowrap">
                        {isEditableArtifact(artifact) && (
                          <button
                            className="copy-id"
                            disabled={busy || !!editorLoading}
                            onClick={() => void openEditor(artifact)}
                          >
                            <FilePenLine size={14} />
                            {editorLoading === artifact.id
                              ? '여는 중…'
                              : '편집'}
                          </button>
                        )}
                        <button
                          className="copy-id"
                          title={artifact.id}
                          onClick={() => void copy(artifact.id)}
                        >
                          <Clipboard size={14} /> ID
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {!artifacts.length && (
              <div className="empty-panel">등록된 파일이 없습니다.</div>
            )}
          </div>
          <p className="artifact-note">
            <Check size={15} /> 아래 매핑에서 토폴로지의 모든 artifactRef를
            확인하고, 적용 후 설계를 저장하세요.
          </p>
        </div>
      </div>
      <ArtifactMappingPanel
        nodes={nodes}
        artifacts={artifacts}
        disabled={!projectId || busy}
        onApply={mapArtifacts}
      />
      {editor && (
        <ArtifactEditorDialog
          artifact={editor.artifact}
          content={editor.content}
          referenceCount={
            analyzeArtifactMappings(nodes, artifacts).filter(
              ({ currentRef }) => currentRef === editor.artifact.id,
            ).length
          }
          onOpenChange={(open) => !open && setEditor(null)}
          onSave={saveEditedArtifact}
        />
      )}
    </section>
  );
}
