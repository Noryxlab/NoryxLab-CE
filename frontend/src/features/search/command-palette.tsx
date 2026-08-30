import * as React from 'react';
import { useNavigate } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import {
  AppWindow,
  Boxes,
  Database,
  FolderGit2,
  KeyRound,
  Network,
  Play,
  Plug,
  Search,
  Terminal,
} from 'lucide-react';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';
import { BareInput } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { SkeletonText } from '@/components/ui/skeleton';
import { searchApi } from '@/lib/api/endpoints';
import { useT, type TranslationKey } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import type { SearchResult } from '@/lib/api/types';

/**
 * Command palette.
 *
 * Finding a dataset previously meant knowing which screen held it. Search is
 * the answer, and a palette is how it gets used: a search box parked on one
 * page is visited deliberately, a palette is reached by reflex from anywhere.
 *
 * Results are scoped server-side by the same rules the listing endpoints use,
 * so this cannot surface anything the user could not already open.
 */

const KIND_LABEL: Record<string, TranslationKey> = {
  project: 'search.kind_project',
  dataset: 'search.kind_dataset',
  datasource: 'search.kind_datasource',
  ontology: 'search.kind_ontology',
  repository: 'search.kind_repository',
  secret: 'search.kind_secret',
  workspace: 'search.kind_workspace',
  job: 'search.kind_job',
  app: 'search.kind_app',
};

const KIND_ICON: Record<string, React.ComponentType<{ className?: string }>> = {
  project: Boxes,
  dataset: Database,
  datasource: Plug,
  ontology: Network,
  repository: FolderGit2,
  secret: KeyRound,
  workspace: Terminal,
  job: Play,
  app: AppWindow,
};

function resultPath(result: SearchResult): string {
  switch (result.kind) {
    case 'project':
      return `/projects/${result.id}`;
    case 'workspace':
      return `/projects/${result.projectId}/workspaces`;
    case 'job':
      return `/projects/${result.projectId}/jobs`;
    case 'app':
      return `/projects/${result.projectId}/apps`;
    case 'dataset':
      return `/catalog/datasets/${result.id}`;
    case 'datasource':
      return '/catalog/datasources';
    case 'ontology':
      return '/catalog/ontologies';
    case 'repository':
      return '/catalog/repositories';
    case 'secret':
      return '/catalog/secrets';
    default:
      return '/';
  }
}

export function CommandPalette() {
  const t = useT();
  const navigate = useNavigate();
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState('');
  const [highlighted, setHighlighted] = React.useState(0);
  const [debounced, setDebounced] = React.useState('');

  // Debounced: typing "ventes" should not fire six searches.
  React.useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(query), 180);
    return () => window.clearTimeout(timer);
  }, [query]);

  React.useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen((current) => !current);
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  React.useEffect(() => {
    if (!open) {
      setQuery('');
      setDebounced('');
      setHighlighted(0);
    }
  }, [open]);

  const search = useQuery({
    queryKey: ['search', debounced],
    queryFn: () => searchApi.search(debounced),
    enabled: open && debounced.trim().length >= 2,
    staleTime: 10_000,
  });

  const results = search.data ?? [];
  React.useEffect(() => setHighlighted(0), [debounced]);

  function open_(result: SearchResult) {
    setOpen(false);
    navigate(resultPath(result));
  }

  function onKeyDown(event: React.KeyboardEvent) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setHighlighted((current) => Math.min(current + 1, results.length - 1));
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setHighlighted((current) => Math.max(current - 1, 0));
    } else if (event.key === 'Enter' && results[highlighted]) {
      event.preventDefault();
      open_(results[highlighted]);
    }
  }

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={cn(
          'flex h-8 items-center gap-2 rounded-md border border-border-strong bg-surface px-2.5 text-xs text-muted-foreground',
          'transition-colors hover:bg-surface-muted',
          'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
        )}
        aria-label={t('search.open')}
      >
        <Search className="size-3.5" aria-hidden />
        <span className="hidden sm:inline">{t('search.placeholder')}</span>
        <kbd className="hidden rounded border border-border bg-surface-muted px-1 font-sans text-[0.6875rem] sm:inline">
          ⌘K
        </kbd>
      </button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent size="md" className="top-[20%] translate-y-0 p-0" aria-describedby={undefined}>
          <DialogTitle className="sr-only">{t('search.title')}</DialogTitle>
          <div className="flex items-center gap-2 border-b border-border px-3 py-2">
            <Search className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            <BareInput
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              onKeyDown={onKeyDown}
              placeholder={t('search.placeholder')}
              aria-label={t('search.title')}
              autoFocus
              className="h-9 border-0 bg-transparent px-0 focus-visible:outline-none"
            />
          </div>

          <div className="scrollbar-thin max-h-96 overflow-y-auto p-2">
            {debounced.trim().length < 2 ? (
              <p className="px-2 py-6 text-center text-xs text-muted-foreground">{t('search.hint')}</p>
            ) : search.isLoading ? (
              <div className="p-2">
                <SkeletonText lines={3} />
              </div>
            ) : results.length === 0 ? (
              <p className="px-2 py-6 text-center text-xs text-muted-foreground">{t('search.empty')}</p>
            ) : (
              <ul role="listbox" aria-label={t('search.title')}>
                {results.map((result, index) => {
                  const Icon = KIND_ICON[result.kind] ?? Search;
                  const kindKey = KIND_LABEL[result.kind];
                  return (
                    <li key={`${result.kind}-${result.id}`}>
                      <button
                        type="button"
                        role="option"
                        aria-selected={index === highlighted}
                        onMouseEnter={() => setHighlighted(index)}
                        onClick={() => open_(result)}
                        className={cn(
                          'flex w-full items-center gap-2.5 rounded-md px-2 py-2 text-left transition-colors',
                          index === highlighted ? 'bg-surface-muted' : 'hover:bg-surface-muted',
                        )}
                      >
                        <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm">{result.label}</span>
                          {result.sublabel ? (
                            <span className="block truncate text-xs text-muted-foreground">
                              {result.sublabel}
                            </span>
                          ) : null}
                        </span>
                        <Badge tone="outline">
                          {kindKey ? t(kindKey) : result.kind}
                        </Badge>
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
