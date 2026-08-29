import * as React from 'react';
import * as LabelPrimitive from '@radix-ui/react-label';
import { AlertCircle } from 'lucide-react';
import { cn } from '@/lib/utils';

/**
 * Form field primitive.
 *
 * The previous UI used placeholders as labels throughout:
 *
 *   <input id="projectName" placeholder="Nom du nouveau projet" />
 *   <input id="workspaceStorageSize" placeholder="Stockage (ex. 10Gi)" />
 *
 * The label disappears as soon as the user types, which fails WCAG 3.3.2 and
 * is the single strongest "unfinished prototype" signal in an interface.
 * `Field` makes the correct structure the path of least resistance: a real
 * <label>, an optional description, and an error that is wired to the control
 * through aria-describedby / aria-invalid rather than a coloured div.
 */

interface FieldContextValue {
  id: string;
  descriptionId: string;
  errorId: string;
  hasError: boolean;
  hasDescription: boolean;
}

const FieldContext = React.createContext<FieldContextValue | null>(null);

function useFieldContext(): FieldContextValue {
  const ctx = React.useContext(FieldContext);
  if (!ctx) throw new Error('Field subcomponents must be used inside <Field>');
  return ctx;
}

/** Wires aria attributes onto whichever control the field wraps. */
export function useFieldControlProps(): {
  id: string;
  'aria-describedby': string | undefined;
  'aria-invalid': boolean | undefined;
} {
  const { id, descriptionId, errorId, hasError, hasDescription } = useFieldContext();
  const describedBy = [hasDescription ? descriptionId : null, hasError ? errorId : null]
    .filter(Boolean)
    .join(' ');
  return {
    id,
    'aria-describedby': describedBy || undefined,
    'aria-invalid': hasError || undefined,
  };
}

export interface FieldProps extends Omit<React.HTMLAttributes<HTMLDivElement>, 'id'> {
  label: React.ReactNode;
  /** Persistent helper text. Prefer this over a placeholder for guidance. */
  description?: React.ReactNode;
  /** Validation message. Presence switches the field into its error state. */
  error?: React.ReactNode;
  required?: boolean;
  /** Renders the label visually hidden while keeping it for screen readers. */
  hideLabel?: boolean;
  htmlFor?: string;
}

export function Field({
  label,
  description,
  error,
  required = false,
  hideLabel = false,
  htmlFor,
  className,
  children,
  ...props
}: FieldProps) {
  const generatedId = React.useId();
  const id = htmlFor ?? generatedId;
  const ctx = React.useMemo<FieldContextValue>(
    () => ({
      id,
      descriptionId: `${id}-description`,
      errorId: `${id}-error`,
      hasError: Boolean(error),
      hasDescription: Boolean(description),
    }),
    [id, error, description],
  );

  return (
    <FieldContext.Provider value={ctx}>
      <div className={cn('flex min-w-0 flex-col gap-1.5', className)} {...props}>
        <LabelPrimitive.Root
          htmlFor={id}
          className={cn(
            'text-xs font-medium text-foreground',
            hideLabel && 'sr-only',
          )}
        >
          {label}
          {required ? (
            <span className="ml-0.5 text-danger" aria-hidden>
              *
            </span>
          ) : null}
        </LabelPrimitive.Root>
        {children}
        {description ? (
          <p id={ctx.descriptionId} className="text-xs leading-relaxed text-muted-foreground">
            {description}
          </p>
        ) : null}
        {error ? (
          <p
            id={ctx.errorId}
            className="flex items-start gap-1.5 text-xs font-medium text-danger"
          >
            <AlertCircle className="mt-px size-3.5 shrink-0" aria-hidden />
            <span>{error}</span>
          </p>
        ) : null}
      </div>
    </FieldContext.Provider>
  );
}

/** Standalone label for controls that are not inside a Field. */
export const Label = React.forwardRef<
  React.ComponentRef<typeof LabelPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof LabelPrimitive.Root>
>(function Label({ className, ...props }, ref) {
  return (
    <LabelPrimitive.Root
      ref={ref}
      className={cn('text-xs font-medium text-foreground', className)}
      {...props}
    />
  );
});

/** Read-only value display, for attributes the user cannot edit.
 *  Replaces the previous `<select disabled>` pattern, which advertised a
 *  choice that did not exist. */
export function ReadOnlyValue({
  label,
  value,
  description,
  className,
}: {
  label: React.ReactNode;
  value: React.ReactNode;
  description?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('flex min-w-0 flex-col gap-1.5', className)}>
      <span className="text-xs font-medium text-foreground">{label}</span>
      <div className="flex h-9 items-center rounded-md border border-dashed border-border bg-surface-muted px-3 text-sm text-muted-foreground">
        {value}
      </div>
      {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
    </div>
  );
}
