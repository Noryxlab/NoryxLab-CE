import * as React from 'react';
import { ArrowDown, ArrowUp, ChevronsUpDown, MoreHorizontal } from 'lucide-react';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrapper,
} from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { SkeletonTable } from '@/components/ui/skeleton';
import { ErrorState, NoResultsState } from './states';
import { cn } from '@/lib/utils';

export interface Column<T> {
  id: string;
  header: React.ReactNode;
  cell: (row: T) => React.ReactNode;
  /** Returns a comparable value; presence enables sorting on this column. */
  sortValue?: (row: T) => string | number | null;
  /** Contributes to free-text search across the table. */
  searchValue?: (row: T) => string | null | undefined;
  className?: string;
  headClassName?: string;
  align?: 'left' | 'right';
}

export interface DataTableProps<T> {
  data: T[] | undefined;
  columns: Column<T>[];
  rowKey: (row: T) => string;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
  onRetry?: () => void;
  /** Rendered when the dataset itself is empty (as opposed to filtered out). */
  emptyState?: React.ReactNode;
  /** Free-text query, owned by the caller so it can live in a toolbar. */
  search?: string;
  onResetSearch?: () => void;
  defaultSort?: { columnId: string; direction: 'asc' | 'desc' };
  onRowClick?: (row: T) => void;
  rowActions?: (row: T) => React.ReactNode;
  className?: string;
}

export function DataTable<T>({
  data,
  columns,
  rowKey,
  isLoading = false,
  isError = false,
  error,
  onRetry,
  emptyState,
  search = '',
  onResetSearch,
  defaultSort,
  onRowClick,
  rowActions,
  className,
}: DataTableProps<T>) {
  const [sort, setSort] = React.useState<{ columnId: string; direction: 'asc' | 'desc' } | null>(
    defaultSort ?? null,
  );

  const filtered = React.useMemo(() => {
    if (!data) return [];
    const query = search.trim().toLowerCase();
    if (!query) return data;
    const searchable = columns.filter((column) => column.searchValue);
    return data.filter((row) =>
      searchable.some((column) => {
        const value = column.searchValue?.(row);
        return typeof value === 'string' && value.toLowerCase().includes(query);
      }),
    );
  }, [data, search, columns]);

  const sorted = React.useMemo(() => {
    if (!sort) return filtered;
    const column = columns.find((candidate) => candidate.id === sort.columnId);
    if (!column?.sortValue) return filtered;
    const factor = sort.direction === 'asc' ? 1 : -1;
    return [...filtered].sort((left, right) => {
      const a = column.sortValue?.(left) ?? null;
      const b = column.sortValue?.(right) ?? null;
      if (a === null && b === null) return 0;
      if (a === null) return 1;
      if (b === null) return -1;
      if (typeof a === 'number' && typeof b === 'number') return (a - b) * factor;
      return String(a).localeCompare(String(b), 'fr', { numeric: true }) * factor;
    });
  }, [filtered, sort, columns]);

  const columnCount = columns.length + (rowActions ? 1 : 0);

  function toggleSort(columnId: string) {
    setSort((current) => {
      if (current?.columnId !== columnId) return { columnId, direction: 'asc' };
      if (current.direction === 'asc') return { columnId, direction: 'desc' };
      return null;
    });
  }

  if (isError) return <ErrorState error={error} onRetry={onRetry} />;
  if (isLoading) return <SkeletonTable columns={Math.min(columnCount, 5)} />;
  if (data && data.length === 0 && emptyState) return <>{emptyState}</>;

  return (
    <TableWrapper className={className}>
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            {columns.map((column) => {
              const sortable = Boolean(column.sortValue);
              const active = sort?.columnId === column.id;
              const SortIcon = !active ? ChevronsUpDown : sort.direction === 'asc' ? ArrowUp : ArrowDown;
              return (
                <TableHead
                  key={column.id}
                  className={cn(column.align === 'right' && 'text-right', column.headClassName)}
                  aria-sort={active ? (sort.direction === 'asc' ? 'ascending' : 'descending') : undefined}
                >
                  {sortable ? (
                    <button
                      type="button"
                      onClick={() => toggleSort(column.id)}
                      className={cn(
                        'inline-flex items-center gap-1 rounded transition-colors hover:text-foreground',
                        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
                        active && 'text-foreground',
                        column.align === 'right' && 'flex-row-reverse',
                      )}
                    >
                      {column.header}
                      <SortIcon className="size-3" aria-hidden />
                    </button>
                  ) : (
                    column.header
                  )}
                </TableHead>
              );
            })}
            {rowActions ? <TableHead className="w-10 text-right sr-only">Actions</TableHead> : null}
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.length === 0 ? (
            <TableRow className="hover:bg-transparent">
              <TableCell colSpan={columnCount} className="p-0">
                <NoResultsState onReset={onResetSearch} />
              </TableCell>
            </TableRow>
          ) : (
            sorted.map((row) => (
              <TableRow
                key={rowKey(row)}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
                tabIndex={onRowClick ? 0 : undefined}
                role={onRowClick ? 'button' : undefined}
                onKeyDown={
                  onRowClick
                    ? (event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          onRowClick(row);
                        }
                      }
                    : undefined
                }
                className={cn(onRowClick && 'cursor-pointer')}
              >
                {columns.map((column) => (
                  <TableCell
                    key={column.id}
                    className={cn(column.align === 'right' && 'text-right', column.className)}
                  >
                    {column.cell(row)}
                  </TableCell>
                ))}
                {rowActions ? (
                  <TableCell className="w-10 text-right" onClick={(event) => event.stopPropagation()}>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon-sm" aria-label="Actions">
                          <MoreHorizontal aria-hidden />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent>{rowActions(row)}</DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                ) : null}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </TableWrapper>
  );
}
