import type * as React from 'react';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

export interface StatProps {
  label: string;
  value: React.ReactNode;
  hint?: React.ReactNode;
  icon?: React.ComponentType<{ className?: string }>;
  loading?: boolean;
  className?: string;
}

export function Stat({ label, value, hint, icon: Icon, loading = false, className }: StatProps) {
  return (
    <div className={cn('rounded-lg border border-border bg-surface p-4', className)}>
      <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
        {Icon ? <Icon className="size-3.5" /> : null}
        <span className="truncate">{label}</span>
      </div>
      {loading ? (
        <Skeleton className="mt-2 h-7 w-20" />
      ) : (
        <p className="mt-1.5 text-2xl font-semibold tracking-tight tabular-nums">{value}</p>
      )}
      {hint ? <p className="mt-1 text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  );
}

export function StatGrid({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('grid gap-3 sm:grid-cols-2 lg:grid-cols-4', className)}
      {...props}
    />
  );
}
