import * as React from 'react';
import { Download, WrapText } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { SearchInput } from './search-input';
import { EmptyState } from './states';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

export interface LogViewerProps {
  content: string | null | undefined;
  isLoading?: boolean;
  /** Filename used when the user downloads the buffer. */
  downloadName?: string;
  emptyLabel?: string;
  className?: string;
}

/**
 * Log surface for jobs, apps, builds and datasources.
 *
 * Replaces the previous `<pre id="jobLogsOut">` dumps, which had no search,
 * no wrapping control and no download, and which sat behind a `<summary>`
 * labelled "Diagnostic" on screens end users could see.
 */
export function LogViewer({
  content,
  isLoading = false,
  downloadName = 'noryx-logs.txt',
  emptyLabel = 'Aucun journal disponible pour le moment.',
  className,
}: LogViewerProps) {
  const [query, setQuery] = React.useState('');
  const [wrap, setWrap] = React.useState(false);

  const lines = React.useMemo(() => (content ? content.split('\n') : []), [content]);
  const visible = React.useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return lines;
    return lines.filter((line) => line.toLowerCase().includes(needle));
  }, [lines, query]);

  function download() {
    const blob = new Blob([content ?? ''], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = downloadName;
    anchor.click();
    URL.revokeObjectURL(url);
  }

  if (isLoading) {
    return (
      <div className={cn('space-y-2 rounded-md border border-border bg-surface-muted p-3', className)}>
        {Array.from({ length: 6 }, (_, index) => (
          <Skeleton key={index} className={index % 3 === 0 ? 'h-3 w-2/3' : 'h-3 w-full'} />
        ))}
      </div>
    );
  }

  if (!content) {
    return (
      <div className={cn('rounded-md border border-border bg-surface-muted', className)}>
        <EmptyState compact title={emptyLabel} />
      </div>
    );
  }

  return (
    <div className={cn('overflow-hidden rounded-md border border-border', className)}>
      <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-muted px-2 py-1.5">
        <SearchInput
          value={query}
          onValueChange={setQuery}
          label="Filtrer les lignes"
          className="h-8 min-w-40 flex-1"
        />
        <span className="text-xs tabular-nums text-muted-foreground">
          {visible.length} / {lines.length} lignes
        </span>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={wrap ? 'Désactiver le retour à la ligne' : 'Activer le retour à la ligne'}
          aria-pressed={wrap}
          onClick={() => setWrap((current) => !current)}
        >
          <WrapText aria-hidden className={wrap ? 'text-brand' : undefined} />
        </Button>
        <Button variant="ghost" size="icon-sm" aria-label="Télécharger les journaux" onClick={download}>
          <Download aria-hidden />
        </Button>
      </div>
      <pre
        className={cn(
          'scrollbar-thin max-h-96 overflow-auto bg-surface px-3 py-2 font-mono text-xs leading-relaxed',
          wrap ? 'whitespace-pre-wrap break-words' : 'whitespace-pre',
        )}
        tabIndex={0}
      >
        {visible.join('\n')}
      </pre>
    </div>
  );
}
