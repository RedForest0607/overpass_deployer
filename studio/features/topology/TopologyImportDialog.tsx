'use client';

import { useEffect, useRef, useState } from 'react';
import { FileUp } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import type { Environment } from '../../lib/api/contracts';
import JsonEditorTextarea from './JsonEditorTextarea';
import {
  MAX_TOPOLOGY_IMPORT_BYTES,
  parseTopologyImport,
  type ImportedTopology,
} from './topology-import';

export default function TopologyImportDialog({
  open,
  ...props
}: ImportDialogProps) {
  return open ? <ImportSession open={open} {...props} /> : null;
}

type ImportDialogProps = {
  open: boolean;
  dirty: boolean;
  environment: Environment;
  onOpenChange: (open: boolean) => void;
  onImport: (value: ImportedTopology, filename?: string) => void;
};

function ImportSession({
  open,
  dirty,
  environment,
  onOpenChange,
  onImport,
}: {
  open: boolean;
  dirty: boolean;
  environment: Environment;
  onOpenChange: (open: boolean) => void;
  onImport: (value: ImportedTopology, filename?: string) => void;
}) {
  const [source, setSource] = useState('');
  const [filename, setFilename] = useState('');
  const [error, setError] = useState('');
  const [pending, setPending] = useState<ImportedTopology | null>(null);
  const fileRead = useRef(0);

  useEffect(() => {
    const token = fileRead;
    return () => {
      token.current += 1;
    };
  }, []);

  async function selectFile(file?: File) {
    const token = ++fileRead.current;
    setError('');
    setPending(null);
    if (!file) return;
    if (file.size > MAX_TOPOLOGY_IMPORT_BYTES) {
      setError('토폴로지 JSON 파일은 2 MiB 이하여야 합니다.');
      return;
    }
    try {
      const contents = await file.text();
      if (token !== fileRead.current) return;
      setSource(contents);
      setFilename(file.name);
    } catch {
      if (token !== fileRead.current) return;
      setError('파일을 읽지 못했습니다. 다시 선택하세요.');
    }
  }

  function prepareImport() {
    setError('');
    try {
      const parsed = parseTopologyImport(source);
      if (parsed.environment && parsed.environment !== environment) {
        throw new Error(
          `파일은 ${parsed.environment.toUpperCase()} 환경입니다. 현재 ${environment.toUpperCase()} 환경으로 전환한 뒤 가져오세요.`,
        );
      }
      if (dirty) {
        setPending(parsed);
        return;
      }
      finishImport(parsed);
    } catch (cause) {
      setPending(null);
      setError(
        cause instanceof Error ? cause.message : '토폴로지를 읽지 못했습니다.',
      );
    }
  }

  function finishImport(value: ImportedTopology) {
    onImport(value, filename || undefined);
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[calc(100dvh-2rem)] flex-col overflow-hidden sm:max-w-3xl">
        <DialogHeader className="shrink-0">
          <DialogTitle>토폴로지 JSON 가져오기</DialogTitle>
          <DialogDescription>
            Studio에서 내보낸 파일을 선택하거나 Blueprint JSON을 붙여 넣으세요.
            가져온 내용은 저장 전까지 로컬 편집 상태로 유지됩니다.
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 overflow-y-auto overscroll-contain pr-1">
          <label className="field">
            <span>JSON 파일</span>
            <input
              type="file"
              accept=".json,application/json"
              onChange={(event) => void selectFile(event.target.files?.[0])}
            />
            <small>
              {filename ||
                '최대 2 MiB · 파일을 선택하면 아래 편집기에 표시됩니다.'}
            </small>
          </label>

          <div className="field">
            <span>JSON 편집기</span>
            <JsonEditorTextarea
              className="h-[min(48dvh,28rem)] min-h-40 max-h-[48dvh]"
              value={source}
              rows={18}
              spellCheck={false}
              aria-invalid={!!error}
              placeholder='{"environment":"dev","blueprint":{...},"layout":{...}}'
              onChange={(event) => {
                setSource(event.target.value);
                setError('');
                setPending(null);
              }}
              expandedTitle="토폴로지 JSON 편집"
              expandedDescription="가져올 Blueprint와 layout JSON을 넓은 화면에서 확인하고 수정합니다."
              onExpandedApply={(value) => {
                try {
                  JSON.parse(value);
                  setSource(value);
                  setError('');
                  setPending(null);
                } catch (cause) {
                  return cause instanceof Error
                    ? cause.message
                    : 'JSON을 확인하세요.';
                }
              }}
            />
          </div>

          {pending && (
            <p role="alert" style={{ color: '#9a5b18' }}>
              현재 미저장 편집 내용이 있습니다. 계속하면 캔버스의 로컬 편집
              내용이 가져온 JSON으로 교체됩니다.
            </p>
          )}
          {error && (
            <p role="alert" style={{ color: '#b34e4e' }}>
              {error}
            </p>
          )}
        </div>

        <DialogFooter className="shrink-0">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            취소
          </Button>
          <Button
            disabled={!source.trim()}
            onClick={() => (pending ? finishImport(pending) : prepareImport())}
          >
            <FileUp /> {pending ? '기존 편집 교체' : '에디터에 적용'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
