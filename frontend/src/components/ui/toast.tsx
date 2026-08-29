import * as React from 'react';
import * as ToastPrimitive from '@radix-ui/react-toast';
import { AlertTriangle, CheckCircle2, Info, X, XCircle } from 'lucide-react';
import { cn } from '@/lib/utils';

export type ToastTone = 'info' | 'success' | 'warning' | 'error';

export interface ToastOptions {
  title?: string;
  description?: string;
  tone?: ToastTone;
  /** Milliseconds. Errors stay longer because they usually need reading. */
  duration?: number;
  action?: { label: string; onClick: () => void };
}

interface ToastRecord extends ToastOptions {
  id: number;
  tone: ToastTone;
}

interface ToastContextValue {
  toast: (options: ToastOptions) => void;
  success: (description: string, title?: string) => void;
  error: (error: unknown, title?: string) => void;
  info: (description: string, title?: string) => void;
}

const ToastContext = React.createContext<ToastContextValue | null>(null);

export function useToast(): ToastContextValue {
  const ctx = React.useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used inside <ToastProvider>');
  return ctx;
}

/** Extracts a readable message out of anything a fetch layer can reject with. */
export function toMessage(error: unknown): string {
  if (error === null || error === undefined) return 'Erreur inconnue.';
  if (typeof error === 'string') return error;
  if (error instanceof Error) return error.message;
  if (typeof error === 'object') {
    const record = error as Record<string, unknown>;
    for (const key of ['error', 'message', 'detail', 'title']) {
      const value = record[key];
      if (typeof value === 'string' && value.trim()) return value;
    }
  }
  return 'Erreur inconnue.';
}

const TONE_ICON: Record<ToastTone, typeof Info> = {
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  error: XCircle,
};

const TONE_CLASS: Record<ToastTone, string> = {
  info: 'text-brand',
  success: 'text-success',
  warning: 'text-warning',
  error: 'text-danger',
};

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = React.useState<ToastRecord[]>([]);
  const nextId = React.useRef(0);

  const dismiss = React.useCallback((id: number) => {
    setItems((current) => current.filter((item) => item.id !== id));
  }, []);

  const toast = React.useCallback((options: ToastOptions) => {
    const id = (nextId.current += 1);
    setItems((current) => [...current.slice(-3), { ...options, id, tone: options.tone ?? 'info' }]);
  }, []);

  const value = React.useMemo<ToastContextValue>(
    () => ({
      toast,
      success: (description, title) => toast({ description, title, tone: 'success' }),
      info: (description, title) => toast({ description, title, tone: 'info' }),
      error: (error, title) =>
        toast({ description: toMessage(error), title, tone: 'error', duration: 9000 }),
    }),
    [toast],
  );

  return (
    <ToastContext.Provider value={value}>
      <ToastPrimitive.Provider swipeDirection="right" duration={5000}>
        {children}
        {items.map((item) => {
          const Icon = TONE_ICON[item.tone];
          return (
            <ToastPrimitive.Root
              key={item.id}
              duration={item.duration}
              onOpenChange={(open) => {
                if (!open) dismiss(item.id);
              }}
              className={cn(
                'flex items-start gap-3 rounded-lg border border-border bg-surface-raised p-3 shadow-lg',
                'data-[state=open]:animate-in data-[state=open]:slide-in-from-right-4',
              )}
            >
              <Icon className={cn('mt-0.5 size-4 shrink-0', TONE_CLASS[item.tone])} aria-hidden />
              <div className="min-w-0 flex-1 space-y-0.5">
                {item.title ? (
                  <ToastPrimitive.Title className="text-sm font-semibold">
                    {item.title}
                  </ToastPrimitive.Title>
                ) : null}
                {item.description ? (
                  <ToastPrimitive.Description className="break-words text-xs leading-relaxed text-muted-foreground">
                    {item.description}
                  </ToastPrimitive.Description>
                ) : null}
                {item.action ? (
                  <ToastPrimitive.Action
                    altText={item.action.label}
                    onClick={item.action.onClick}
                    className="mt-1 text-xs font-medium text-brand underline-offset-4 hover:underline"
                  >
                    {item.action.label}
                  </ToastPrimitive.Action>
                ) : null}
              </div>
              <ToastPrimitive.Close
                aria-label="Fermer"
                className="rounded p-0.5 text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              >
                <X className="size-3.5" aria-hidden />
              </ToastPrimitive.Close>
            </ToastPrimitive.Root>
          );
        })}
        <ToastPrimitive.Viewport className="fixed bottom-0 right-0 z-100 flex w-full max-w-sm flex-col gap-2 p-4 outline-none" />
      </ToastPrimitive.Provider>
    </ToastContext.Provider>
  );
}
