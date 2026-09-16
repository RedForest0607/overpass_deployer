import { useState, type ComponentProps } from 'react';
import { Maximize2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import JsonEditorDialog from './JsonEditorDialog';

type JsonEditorTextareaProps = Omit<
  ComponentProps<typeof Textarea>,
  'value'
> & {
  value: string;
  expandedTitle?: string;
  expandedDescription?: string;
  onExpandedApply?: (value: string) => string | void;
};

export default function JsonEditorTextarea({
  className,
  value,
  expandedTitle = 'JSON 큰 화면 편집기',
  expandedDescription = '넓은 화면에서 JSON을 수정한 뒤 편집 내용을 적용하세요.',
  onExpandedApply,
  ...props
}: JsonEditorTextareaProps) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="relative">
      <Textarea
        className={cn(
          'field-sizing-fixed resize-y overflow-auto font-mono',
          onExpandedApply && 'pr-28',
          className,
        )}
        value={value}
        {...props}
      />
      {onExpandedApply && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="absolute top-2 right-2 bg-background/95 shadow-sm"
          aria-label="JSON을 큰 화면에서 편집"
          onClick={() => setExpanded(true)}
        >
          <Maximize2 /> 크게 편집
        </Button>
      )}
      {onExpandedApply && expanded && (
        <JsonEditorDialog
          open
          title={expandedTitle}
          description={expandedDescription}
          value={value}
          onOpenChange={setExpanded}
          onApply={onExpandedApply}
        />
      )}
    </div>
  );
}
