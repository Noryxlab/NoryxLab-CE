import { Link } from 'react-router';
import { ChevronRight } from 'lucide-react';
import { cn } from '@/lib/utils';

export interface Crumb {
  label: string;
  to?: string;
}

export function Breadcrumbs({ items, className }: { items: Crumb[]; className?: string }) {
  return (
    <nav aria-label="Fil d'Ariane" className={cn('min-w-0', className)}>
      <ol className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
        {items.map((item, index) => {
          const last = index === items.length - 1;
          return (
            <li key={`${item.label}-${index}`} className="flex min-w-0 items-center gap-1">
              {item.to && !last ? (
                <Link
                  to={item.to}
                  className="truncate rounded transition-colors hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                >
                  {item.label}
                </Link>
              ) : (
                <span className={cn('truncate', last && 'font-medium text-foreground')} aria-current={last ? 'page' : undefined}>
                  {item.label}
                </span>
              )}
              {last ? null : <ChevronRight className="size-3 shrink-0" aria-hidden />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
