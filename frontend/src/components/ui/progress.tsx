import type * as React from 'react';
import { cn } from '@/lib/utils';

export interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
  /** 0-100. Null renders an indeterminate bar. */
  value: number | null;
  label?: string;
}

export function Progress({ value, label, className, ...props }: ProgressProps) {
  const clamped = value === null ? null : Math.min(100, Math.max(0, value));
  return (
    <div
      role="progressbar"
      aria-valuenow={clamped ?? undefined}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={label}
      className={cn('h-1.5 w-full overflow-hidden rounded-full bg-surface-muted', className)}
      {...props}
    >
      <div
        className={cn(
          'h-full rounded-full bg-brand transition-[width] duration-300',
          clamped === null && 'w-1/3 motion-safe:animate-pulse',
        )}
        style={clamped === null ? undefined : { width: `${clamped}%` }}
      />
    </div>
  );
}
