'use client';

import { useState } from 'react';
import { FilePenLine, Save } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/textarea';
import type { Artifact } from '../../lib/api/delivery';
import { validateArtifactText } from './artifact-text';

export default function ArtifactEditorDialog({
  artifact,
  content,
  referenceCount,
  onOpenChange,
  onSave,
}: {
  artifact: Artifact;
  content: string;
  referenceCount: number;
  onOpenChange: (open: boolean) => void;
  onSave: (
    content: string,
    replaceReferences: boolean,
  ) => Promise<string | void>;
}) {
  const [draft, setDraft] = useState(content);
  const [replaceReferences, setReplaceReferences] = useState(
    referenceCount > 0,
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  async function save() {
    const validationError = validateArtifactText(artifact.originalName, draft);
    if (validationError) {
      setError(validationError);
      return;
    }
    setSaving(true);
    setError('');
    try {
      const message = await onSave(draft, replaceReferences);
      if (message) {
        setError(message);
        return;
      }
      onOpenChange(false);
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : '파일을 저장하지 못했습니다.',
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !saving && onOpenChange(open)}>
      <DialogContent className="flex h-[calc(100dvh-2rem)] max-h-[calc(100dvh-2rem)] w-[calc(100vw-2rem)] max-w-none flex-col overflow-hidden p-5 sm:max-w-[calc(100vw-3rem)]">
        <DialogHeader className="shrink-0 pr-10">
          <DialogTitle className="flex items-center gap-2">
            <FilePenLine /> {artifact.originalName}
          </DialogTitle>
          <DialogDescription>
            저장하면 원본을 유지한 채 SHA-256 기반 새 artifact 버전을
            등록합니다.
          </DialogDescription>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 flex-col gap-2">
          <div className="flex shrink-0 items-center justify-between gap-3 text-xs text-muted-foreground">
            <span>
              {lineCount(draft).toLocaleString()}줄 ·{' '}
              {draft.length.toLocaleString()}자
            </span>
            <code>{artifact.kind}</code>
          </div>
          <Textarea
            className="field-sizing-fixed min-h-0 flex-1 resize-none overflow-auto whitespace-pre font-mono text-[13px] leading-6"
            value={draft}
            spellCheck={false}
            disabled={saving}
            aria-invalid={!!error}
            onChange={(event) => {
              setDraft(event.target.value);
              setError('');
            }}
            onKeyDown={(event) => {
              if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
                event.preventDefault();
                void save();
              }
            }}
          />
          <div className="flex min-h-6 shrink-0 items-center justify-between gap-4 text-xs">
            {error ? (
              <span role="alert" className="text-destructive">
                {error}
              </span>
            ) : (
              <span className="text-muted-foreground">
                Ctrl/⌘ + Enter로 저장할 수 있습니다.
              </span>
            )}
            <span className="shrink-0 text-muted-foreground">
              {artifact.sha256.slice(0, 12)}…
            </span>
          </div>
          <label className="flex shrink-0 items-start gap-2 rounded-lg border bg-muted/40 p-3 text-sm">
            <input
              type="checkbox"
              className="mt-1"
              checked={replaceReferences}
              disabled={!referenceCount || saving}
              onChange={(event) => setReplaceReferences(event.target.checked)}
            />
            <span>
              현재 토폴로지의 참조 {referenceCount}개를 새 artifact로 교체
              <small className="mt-1 block tracking-normal">
                교체한 토폴로지는 파일 저장 후 설계 저장을 눌러야 확정됩니다.
              </small>
            </span>
          </label>
        </div>

        <DialogFooter className="shrink-0">
          <Button
            type="button"
            variant="outline"
            disabled={saving}
            onClick={() => onOpenChange(false)}
          >
            취소
          </Button>
          <Button type="button" disabled={saving} onClick={() => void save()}>
            <Save /> {saving ? '새 버전 저장 중…' : '새 버전 저장'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function lineCount(value: string) {
  return value ? value.split('\n').length : 0;
}
