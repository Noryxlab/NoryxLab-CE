import * as React from 'react';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';

export interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  loading?: boolean;
  /** When set, the user must type this exact value to enable confirmation.
   *  Reserved for irreversible deletions of named resources. */
  confirmationValue?: string;
  confirmationLabel?: React.ReactNode;
  onConfirm: () => void;
}


export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = 'Confirmer',
  cancelLabel = 'Annuler',
  destructive = false,
  loading = false,
  confirmationValue,
  confirmationLabel,
  onConfirm,
}: ConfirmDialogProps) {
  const [typed, setTyped] = React.useState('');

  React.useEffect(() => {
    if (!open) setTyped('');
  }, [open]);

  const canConfirm = !confirmationValue || typed.trim() === confirmationValue;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="sm">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>
        {confirmationValue ? (
          <DialogBody>
            <Field
              label={confirmationLabel ?? 'Confirmation'}
              description={
                <>
                  Saisissez <code className="font-mono text-foreground">{confirmationValue}</code> pour
                  confirmer.
                </>
              }
            >
              <Input
                value={typed}
                onChange={(event) => setTyped(event.target.value)}
                autoComplete="off"
                autoFocus
              />
            </Field>
          </DialogBody>
        ) : null}
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={loading}>
            {cancelLabel}
          </Button>
          <Button
            variant={destructive ? 'danger' : 'primary'}
            onClick={onConfirm}
            disabled={!canConfirm}
            loading={loading}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** Imperative confirmation, for action menus where wiring local state per
 *  item would be noise. Returns a element to render plus an `ask` function. */
export function useConfirm(): {
  dialog: React.ReactElement | null;
  ask: (options: Omit<ConfirmDialogProps, 'open' | 'onOpenChange' | 'onConfirm'> & {
    onConfirm: () => unknown;
  }) => void;
} {
  const [pending, setPending] = React.useState<
    | (Omit<ConfirmDialogProps, 'open' | 'onOpenChange' | 'onConfirm'> & {
        onConfirm: () => unknown;
      })
    | null
  >(null);
  const [busy, setBusy] = React.useState(false);

  const ask = React.useCallback((options: Parameters<ReturnType<typeof useConfirm>['ask']>[0]) => {
    setPending(options);
  }, []);

  const dialog = pending ? (
    <ConfirmDialog
      {...pending}
      open
      loading={busy}
      onOpenChange={(open) => {
        if (!open && !busy) setPending(null);
      }}
      onConfirm={() => {
        setBusy(true);
        void Promise.resolve(pending.onConfirm()).finally(() => {
          setBusy(false);
          setPending(null);
        });
      }}
    />
  ) : null;

  return { dialog, ask };
}
