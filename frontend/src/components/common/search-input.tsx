import type * as React from 'react';
import { Search, X } from 'lucide-react';
import { BareInput } from '@/components/ui/input';
import { cn } from '@/lib/utils';

export interface SearchInputProps
  extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange'> {
  value: string;
  onValueChange: (value: string) => void;
  label: string;
}

export function SearchInput({
  value,
  onValueChange,
  label,
  className,
  ...props
}: SearchInputProps) {
  return (
    <div className={cn('relative min-w-0', className)}>
      <Search
        className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
        aria-hidden
      />
      <BareInput
        type="search"
        aria-label={label}
        placeholder={label}
        value={value}
        onChange={(event) => onValueChange(event.target.value)}
        className="px-8 [&::-webkit-search-cancel-button]:appearance-none"
        {...props}
      />
      {value ? (
        <button
          type="button"
          onClick={() => onValueChange('')}
          aria-label="Effacer la recherche"
          className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded p-1 text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          <X className="size-3.5" aria-hidden />
        </button>
      ) : null}
    </div>
  );
}
