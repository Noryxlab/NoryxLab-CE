import type * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const badgeVariants = cva(
  'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap',
  {
    variants: {
      tone: {
        neutral: 'bg-neutral-subtle text-neutral-subtle-foreground',
        brand: 'bg-brand-subtle text-brand-subtle-foreground',
        success: 'bg-success-subtle text-success-foreground',
        warning: 'bg-warning-subtle text-warning-foreground',
        danger: 'bg-danger-subtle text-danger-foreground',
        outline: 'border border-border-strong text-muted-foreground',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, tone, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}

export type StatusTone = 'neutral' | 'brand' | 'success' | 'warning' | 'danger';

export interface StatusDescriptor {
  tone: StatusTone;
  label: string;
  /** True while the resource is converging towards a terminal state. */
  pending: boolean;
}

/**
 * Maps a backend status string onto a tone and a human label.
 *
 * The previous UI did substring matching inline at the call site
 * (`s.includes('run') || s.includes('complete')`) and rendered the raw
 * Kubernetes phase, so users saw "ContainerCreating" and "Terminating".
 * Phases are an implementation detail; the label here is what an analyst
 * needs to know: is it usable, is it working on it, did it fail.
 */
export function describeStatus(status: string | null | undefined, locale: 'fr' | 'en' = 'fr'): StatusDescriptor {
  const raw = String(status ?? '').trim().toLowerCase();
  const fr = locale === 'fr';

  if (!raw) return { tone: 'neutral', label: fr ? 'Inconnu' : 'Unknown', pending: false };

  if (/(^|[^a-z])(running|active|ready|available|healthy|succeeded|success|complete|completed|published)/.test(raw)) {
    const done = /(succeeded|success|complete|completed)/.test(raw);
    return {
      tone: 'success',
      label: done ? (fr ? 'Terminé' : 'Completed') : fr ? 'En marche' : 'Running',
      pending: false,
    };
  }

  if (/(failed|error|crash|backoff|imagepull|evicted|unhealthy|timeout|cancelled|canceled)/.test(raw)) {
    const cancelled = /(cancelled|canceled)/.test(raw);
    return {
      tone: 'danger',
      label: cancelled ? (fr ? 'Annulé' : 'Cancelled') : fr ? 'En échec' : 'Failed',
      pending: false,
    };
  }

  if (/(pending|creating|building|submitted|queued|scheduling|initializing|starting|progress)/.test(raw)) {
    const building = /build/.test(raw);
    return {
      tone: 'warning',
      label: building ? (fr ? 'Construction' : 'Building') : fr ? 'Démarrage' : 'Starting',
      pending: true,
    };
  }

  if (/(stopped|stopping|terminated|terminating|deleted|deleting|suspended|paused)/.test(raw)) {
    const inFlight = /(ing)$/.test(raw);
    return {
      tone: 'neutral',
      label: inFlight ? (fr ? 'Arrêt en cours' : 'Stopping') : fr ? 'Arrêté' : 'Stopped',
      pending: inFlight,
    };
  }

  return { tone: 'neutral', label: status ?? '', pending: false };
}

export interface StatusBadgeProps {
  status: string | null | undefined;
  locale?: 'fr' | 'en';
  /** Shows the raw backend phase in a tooltip-style title, for operators. */
  showRaw?: boolean;
  className?: string;
}

export function StatusBadge({ status, locale = 'fr', showRaw = true, className }: StatusBadgeProps) {
  const descriptor = describeStatus(status, locale);
  return (
    <Badge
      tone={descriptor.tone}
      className={className}
      title={showRaw && status ? String(status) : undefined}
    >
      <span
        className={cn(
          'size-1.5 rounded-full bg-current',
          descriptor.pending && 'motion-safe:animate-pulse',
        )}
        aria-hidden
      />
      {descriptor.label}
    </Badge>
  );
}

export { badgeVariants };
