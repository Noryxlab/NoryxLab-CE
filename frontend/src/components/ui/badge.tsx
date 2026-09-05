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

  // A degraded run completed but did not fulfil its contract - an incomplete
  // backup, for instance. It must never read as a success (ADR-034).
  if (/(degraded|partial|incomplete)/.test(raw)) {
    return { tone: 'warning', label: fr ? 'Incomplet' : 'Incomplete', pending: false };
  }

  if (/(failed|error|crash|backoff|imagepull|evicted|unhealthy|timeout|cancelled|canceled)/.test(raw)) {
    const cancelled = /(cancelled|canceled)/.test(raw);
    return {
      tone: 'danger',
      label: cancelled ? (fr ? 'Annulé' : 'Cancelled') : fr ? 'En échec' : 'Failed',
      pending: false,
    };
  }

  // "launching" is the status the backend gives a workspace between its pod
  // being created and its service answering. It was in none of these lists, so
  // it read as neither pending nor anything else: the list stopped polling and
  // the screen stayed on "launching" until somebody reloaded the page, while
  // the workspace had in fact started.
  if (/(pending|creating|building|launching|submitted|queued|scheduling|initializing|starting|progress)/.test(raw)) {
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

  // A status the platform has no answer for yet - a check that has not run, a
  // resource it cannot see. Not a failure: it says nothing about the thing.
  if (/(unchecked|missing|unknown)/.test(raw)) {
    return { tone: 'neutral', label: fr ? 'Inconnu' : 'Unknown', pending: false };
  }

  // Anything else is a status this interface has never heard of, and the
  // honest reading is "still working on it": keeping `pending` true keeps the
  // list polling, so a screen recovers on its own instead of freezing on a
  // word - which is exactly what "launching" did. The warning is for whoever
  // added the status without declaring it; backend/internal/domain/status is
  // the list, and a test there fails when the two drift.
  if (typeof console !== 'undefined') {
    console.warn(`Unknown status "${raw}": treating it as in progress. Declare it in backend/internal/domain/status.`);
  }
  return { tone: 'neutral', label: status ?? '', pending: true };
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
