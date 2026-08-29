import type * as React from 'react';
import { AlertTriangle, Inbox, RotateCw, SearchX } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { toMessage } from '@/components/ui/toast';
import { cn } from '@/lib/utils';

/**
 * Empty, error and no-results states.
 *
 * The previous UI wrote these inline as a single table cell: `Aucun detail.`,
 * `Aucune execution trouvee.`, or just `-`. An empty state is the best
 * onboarding moment a product gets, so each one here carries a title, one
 * explanatory sentence, and the action that resolves it.
 */

export interface EmptyStateProps {
  icon?: React.ComponentType<{ className?: string }>;
  title: string;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
  /** Compact variant for use inside a table body. */
  compact?: boolean;
}

export function EmptyState({
  icon: Icon = Inbox,
  title,
  description,
  action,
  className,
  compact = false,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-2 text-center',
        compact ? 'px-4 py-8' : 'px-6 py-14',
        className,
      )}
    >
      <div className="rounded-full border border-border bg-surface-muted p-2.5 text-muted-foreground">
        <Icon className={compact ? 'size-4' : 'size-5'} />
      </div>
      <p className="text-sm font-medium text-foreground">{title}</p>
      {description ? (
        <p className="max-w-sm text-xs leading-relaxed text-muted-foreground">{description}</p>
      ) : null}
      {action ? <div className="pt-1.5">{action}</div> : null}
    </div>
  );
}

export function NoResultsState({
  onReset,
  label = 'Aucun résultat',
  description = 'Aucun élément ne correspond à votre recherche. Essayez d’élargir les filtres.',
  resetLabel = 'Réinitialiser les filtres',
}: {
  onReset?: () => void;
  label?: string;
  description?: string;
  resetLabel?: string;
}) {
  return (
    <EmptyState
      compact
      icon={SearchX}
      title={label}
      description={description}
      action={
        onReset ? (
          <Button variant="secondary" size="sm" onClick={onReset}>
            {resetLabel}
          </Button>
        ) : null
      }
    />
  );
}

export interface ErrorStateProps {
  error: unknown;
  onRetry?: () => void;
  title?: string;
  className?: string;
  compact?: boolean;
  retryLabel?: string;
}

/** Per-section error surface. Failures stay scoped to the card that failed
 *  rather than blanking the page. */
export function ErrorState({
  error,
  onRetry,
  title = 'Chargement impossible',
  className,
  compact = false,
  retryLabel = 'Réessayer',
}: ErrorStateProps) {
  return (
    <div
      role="alert"
      className={cn(
        'flex flex-col items-center justify-center gap-2 text-center',
        compact ? 'px-4 py-8' : 'px-6 py-12',
        className,
      )}
    >
      <div className="rounded-full border border-danger/30 bg-danger-subtle p-2.5 text-danger">
        <AlertTriangle className={compact ? 'size-4' : 'size-5'} />
      </div>
      <p className="text-sm font-medium text-foreground">{title}</p>
      <p className="max-w-md break-words text-xs leading-relaxed text-muted-foreground">
        {toMessage(error)}
      </p>
      {onRetry ? (
        <Button variant="secondary" size="sm" onClick={onRetry} className="mt-1.5">
          <RotateCw aria-hidden />
          {retryLabel}
        </Button>
      ) : null}
    </div>
  );
}

/**
 * Renders the right state for a query: loading, error, empty, or content.
 * Centralising this is what guarantees no screen silently shows nothing.
 */
export interface QueryBoundaryProps<T> {
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  data: T | undefined;
  onRetry?: () => void;
  loadingFallback: React.ReactNode;
  emptyFallback?: React.ReactNode;
  isEmpty?: (data: T) => boolean;
  children: (data: T) => React.ReactNode;
}

export function QueryBoundary<T>({
  isLoading,
  isError,
  error,
  data,
  onRetry,
  loadingFallback,
  emptyFallback,
  isEmpty,
  children,
}: QueryBoundaryProps<T>) {
  if (isLoading) return <>{loadingFallback}</>;
  if (isError) return <ErrorState error={error} onRetry={onRetry} />;
  if (data === undefined) return <>{loadingFallback}</>;
  if (emptyFallback && isEmpty?.(data)) return <>{emptyFallback}</>;
  return <>{children(data)}</>;
}
