'use client';

import { useState } from 'react';
import { Braces } from 'lucide-react';
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

export default function JsonEditorDialog({
  open,
  title,
  description,
  value,
  onOpenChange,
  onApply,
}: {
  open: boolean;
  title: string;
  description: string;
  value: string;
  onOpenChange: (open: boolean) => void;
  onApply: (value: string) => string | void;
}) {
  const [draft, setDraft] = useState(value);
  const [error, setError] = useState('');

  function formatJSON() {
    try {
      setDraft(JSON.stringify(JSON.parse(draft), null, 2));
      setError('');
    } catch (cause) {
      setError(errorMessage(cause));
    }
  }

  function apply() {
    try {
      const message = onApply(draft);
      if (message) {
        setError(message);
        return;
      }
      onOpenChange(false);
    } catch (cause) {
      setError(errorMessage(cause));
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[calc(100dvh-2rem)] max-h-[calc(100dvh-2rem)] w-[calc(100vw-2rem)] max-w-none flex-col overflow-hidden p-5 sm:max-w-[calc(100vw-3rem)]">
        <DialogHeader className="shrink-0 pr-10">
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 flex-col gap-2">
          <div className="flex shrink-0 items-center justify-between gap-3">
            <span className="text-xs text-muted-foreground">
              {lineCount(draft).toLocaleString()}줄 ·{' '}
              {draft.length.toLocaleString()}자
            </span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={formatJSON}
            >
              <Braces /> JSON 정렬
            </Button>
          </div>
          <Textarea
            className="field-sizing-fixed min-h-0 flex-1 resize-none overflow-auto whitespace-pre font-mono text-[13px] leading-6"
            value={draft}
            spellCheck={false}
            aria-invalid={!!error}
            onChange={(event) => {
              setDraft(event.target.value);
              setError('');
            }}
            onKeyDown={(event) => {
              if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
                event.preventDefault();
                apply();
              }
            }}
          />
          <div className="min-h-5 shrink-0 text-xs" aria-live="polite">
            {error ? (
              <span role="alert" className="text-destructive">
                {error}
              </span>
            ) : (
              <span className="text-muted-foreground">
                Ctrl/⌘ + Enter로 적용할 수 있습니다.
              </span>
            )}
          </div>
        </div>

        <DialogFooter className="shrink-0">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            취소
          </Button>
          <Button type="button" onClick={apply}>
            편집 내용 적용
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function lineCount(value: string) {
  return value ? value.split('\n').length : 0;
}

function errorMessage(cause: unknown) {
  return cause instanceof Error ? cause.message : 'JSON을 확인하세요.';
}
