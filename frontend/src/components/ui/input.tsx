import * as React from 'react';
import { cn } from '@/lib/utils';
import { useFieldControlProps } from './field';

const controlClasses = cn(
  'w-full rounded-md border border-border-strong bg-surface text-sm text-foreground',
  'transition-colors',
  'focus-visible:outline-2 focus-visible:outline-offset-0 focus-visible:outline-ring focus-visible:border-brand',
  'disabled:cursor-not-allowed disabled:bg-surface-muted disabled:text-muted-foreground',
  'aria-[invalid=true]:border-danger aria-[invalid=true]:outline-danger',
);

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

/** Input bound to the surrounding <Field> for id and aria wiring. */
export const Input = React.forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, ...props },
  ref,
) {
  const field = useFieldControlProps();
  return (
    <input
      ref={ref}
      {...field}
      className={cn(controlClasses, 'h-9 px-3 py-1.5', className)}
      {...props}
    />
  );
});

/** Input for use outside a <Field> (search boxes, inline filters). */
export const BareInput = React.forwardRef<HTMLInputElement, InputProps>(function BareInput(
  { className, ...props },
  ref,
) {
  return <input ref={ref} className={cn(controlClasses, 'h-9 px-3 py-1.5', className)} {...props} />;
});

export type TextareaProps = React.TextareaHTMLAttributes<HTMLTextAreaElement>;

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, ...props },
  ref,
) {
  const field = useFieldControlProps();
  return (
    <textarea
      ref={ref}
      {...field}
      className={cn(controlClasses, 'min-h-20 resize-y px-3 py-2 leading-relaxed', className)}
      {...props}
    />
  );
});

/** Monospace editor for Dockerfiles, commands and configuration payloads. */
export const CodeTextarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  function CodeTextarea({ className, spellCheck = false, ...props }, ref) {
    const field = useFieldControlProps();
    return (
      <textarea
        ref={ref}
        {...field}
        spellCheck={spellCheck}
        className={cn(
          controlClasses,
          'min-h-64 resize-y px-3 py-2 font-mono text-xs leading-relaxed',
          className,
        )}
        {...props}
      />
    );
  },
);
