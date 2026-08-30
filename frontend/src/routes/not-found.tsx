import { Link } from 'react-router';
import { FileQuestion } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { EmptyState } from '@/components/common/states';
import { useT } from '@/lib/i18n';

export function NotFoundPage() {
  const t = useT();
  return (
    <Card className="mx-auto max-w-lg">
      <EmptyState
        icon={FileQuestion}
        title={t('errors.notFoundTitle')}
        description={t('errors.notFoundHint')}
        action={
          <Button variant="primary" asChild>
            <Link to="/">{t('errors.backHome')}</Link>
          </Button>
        }
      />
    </Card>
  );
}
