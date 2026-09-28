import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';

export default function UnsavedChangesDialog({
  open,
  busy,
  currentLabel,
  targetLabel,
  status,
  onCancel,
  onDiscard,
  onSave,
}: {
  open: boolean;
  busy: boolean;
  currentLabel: string;
  targetLabel: string;
  status?: string;
  onCancel: () => void;
  onDiscard: () => void;
  onSave: () => void;
}) {
  return (
    <AlertDialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && !busy) onCancel();
      }}
    >
      <AlertDialogContent className="unsaved-navigation-dialog">
        <AlertDialogHeader>
          <AlertDialogTitle>변경사항을 저장하고 이동할까요?</AlertDialogTitle>
          <AlertDialogDescription>
            {currentLabel}의 미저장 변경이 있습니다. {targetLabel} 대상으로
            이동하기 전에 저장하거나 변경사항을 버려야 합니다.
            {status && (
              <span className="text-destructive block pt-2" role="alert">
                {status}
              </span>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>취소</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={busy}
            onClick={onDiscard}
          >
            저장하지 않고 이동
          </AlertDialogAction>
          <AlertDialogAction disabled={busy} onClick={onSave}>
            {busy ? '처리 중…' : '저장하고 이동'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
