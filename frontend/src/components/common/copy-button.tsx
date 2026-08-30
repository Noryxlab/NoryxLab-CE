import * as React from 'react';
import { Check, Copy } from 'lucide-react';
import { Button, type ButtonProps } from '@/components/ui/button';

export interface CopyButtonProps extends Omit<ButtonProps, 'onClick' | 'children'> {
  value: string;
  label?: string;
}

export function CopyButton({ value, label = 'Copier', ...props }: CopyButtonProps) {
  const [copied, setCopied] = React.useState(false);

  React.useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1800);
    return () => window.clearTimeout(timer);
  }, [copied]);

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={copied ? 'Copié' : label}
      onClick={() => {
        void navigator.clipboard?.writeText(value).then(() => setCopied(true));
      }}
      {...props}
    >
      {copied ? <Check aria-hidden className="text-success" /> : <Copy aria-hidden />}
    </Button>
  );
}
