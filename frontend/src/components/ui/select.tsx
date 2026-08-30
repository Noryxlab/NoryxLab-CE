import * as React from 'react';
import * as SelectPrimitive from '@radix-ui/react-select';
import { Check, ChevronDown, ChevronUp } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useFieldControlProps } from './field';

export const SelectRoot = SelectPrimitive.Root;
export const SelectGroup = SelectPrimitive.Group;
export const SelectValue = SelectPrimitive.Value;

export const SelectTrigger = React.forwardRef<
  React.ComponentRef<typeof SelectPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Trigger>
>(function SelectTrigger({ className, children, ...props }, ref) {
  return (
    <SelectPrimitive.Trigger
      ref={ref}
      className={cn(
        'flex h-9 w-full items-center justify-between gap-2 rounded-md border border-border-strong bg-surface px-3 py-1.5 text-sm text-foreground',
        'transition-colors',
        'focus-visible:outline-2 focus-visible:outline-offset-0 focus-visible:outline-ring focus-visible:border-brand',
        'disabled:cursor-not-allowed disabled:bg-surface-muted disabled:text-muted-foreground',
        'aria-[invalid=true]:border-danger',
        'data-[placeholder]:text-placeholder',
        '[&>span]:min-w-0 [&>span]:truncate [&>span]:text-left',
        className,
      )}
      {...props}
    >
      {children}
      <SelectPrimitive.Icon asChild>
        <ChevronDown className="size-4 shrink-0 text-muted-foreground" aria-hidden />
      </SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
  );
});

export const SelectContent = React.forwardRef<
  React.ComponentRef<typeof SelectPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Content>
>(function SelectContent({ className, children, position = 'popper', ...props }, ref) {
  return (
    <SelectPrimitive.Portal>
      <SelectPrimitive.Content
        ref={ref}
        position={position}
        className={cn(
          'relative z-50 max-h-96 min-w-[8rem] overflow-hidden rounded-md border border-border bg-surface-raised text-foreground shadow-lg',
          'data-[state=open]:animate-in data-[state=open]:fade-in-0',
          position === 'popper' && 'w-full min-w-[var(--radix-select-trigger-width)]',
          className,
        )}
        {...props}
      >
        <SelectPrimitive.ScrollUpButton className="flex h-6 items-center justify-center bg-surface-raised">
          <ChevronUp className="size-4" aria-hidden />
        </SelectPrimitive.ScrollUpButton>
        <SelectPrimitive.Viewport className="p-1">{children}</SelectPrimitive.Viewport>
        <SelectPrimitive.ScrollDownButton className="flex h-6 items-center justify-center bg-surface-raised">
          <ChevronDown className="size-4" aria-hidden />
        </SelectPrimitive.ScrollDownButton>
      </SelectPrimitive.Content>
    </SelectPrimitive.Portal>
  );
});

export const SelectLabel = React.forwardRef<
  React.ComponentRef<typeof SelectPrimitive.Label>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Label>
>(function SelectLabel({ className, ...props }, ref) {
  return (
    <SelectPrimitive.Label
      ref={ref}
      className={cn('px-2 py-1.5 text-xs font-semibold text-muted-foreground', className)}
      {...props}
    />
  );
});

export interface SelectItemProps
  extends React.ComponentPropsWithoutRef<typeof SelectPrimitive.Item> {
  /** Secondary line, e.g. "4 vCPU · 16 Go" under a tier name. */
  hint?: React.ReactNode;
}

export const SelectItem = React.forwardRef<
  React.ComponentRef<typeof SelectPrimitive.Item>,
  SelectItemProps
>(function SelectItem({ className, children, hint, ...props }, ref) {
  return (
    <SelectPrimitive.Item
      ref={ref}
      className={cn(
        'relative flex w-full cursor-default select-none flex-col gap-0.5 rounded-sm py-1.5 pl-8 pr-2 text-sm outline-none',
        'focus:bg-surface-muted data-[state=checked]:bg-brand-subtle data-[state=checked]:text-brand-subtle-foreground',
        'data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
        className,
      )}
      {...props}
    >
      <span className="absolute left-2 top-1.5 flex size-4 items-center justify-center">
        <SelectPrimitive.ItemIndicator>
          <Check className="size-3.5" aria-hidden />
        </SelectPrimitive.ItemIndicator>
      </span>
      <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
      {hint ? <span className="text-xs text-muted-foreground">{hint}</span> : null}
    </SelectPrimitive.Item>
  );
});

export const SelectSeparator = React.forwardRef<
  React.ComponentRef<typeof SelectPrimitive.Separator>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Separator>
>(function SelectSeparator({ className, ...props }, ref) {
  return (
    <SelectPrimitive.Separator ref={ref} className={cn('-mx-1 my-1 h-px bg-border', className)} {...props} />
  );
});

export interface SelectOption {
  value: string;
  label: string;
  hint?: string;
  disabled?: boolean;
}

export interface SelectProps {
  value: string;
  onValueChange: (value: string) => void;
  options: SelectOption[];
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  'aria-label'?: string;
}

/** Field-bound select for the common "pick one from a list" case. */
export function Select({
  value,
  onValueChange,
  options,
  placeholder,
  disabled,
  className,
  ...rest
}: SelectProps) {
  const field = useFieldControlProps();
  return (
    <SelectRoot value={value} onValueChange={onValueChange} disabled={disabled}>
      <SelectTrigger
        id={field.id}
        aria-describedby={field['aria-describedby']}
        aria-invalid={field['aria-invalid']}
        className={className}
        {...rest}
      >
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value} hint={option.hint} disabled={option.disabled}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </SelectRoot>
  );
}

/** Select for use outside a <Field>. */
export function BareSelect({
  value,
  onValueChange,
  options,
  placeholder,
  disabled,
  className,
  ...rest
}: SelectProps) {
  return (
    <SelectRoot value={value} onValueChange={onValueChange} disabled={disabled}>
      <SelectTrigger className={className} {...rest}>
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value} hint={option.hint} disabled={option.disabled}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </SelectRoot>
  );
}
