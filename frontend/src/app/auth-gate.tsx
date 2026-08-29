import { Outlet } from 'react-router';
import { LogIn, ShieldAlert } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useAuth } from '@/lib/auth';
import { useT } from '@/lib/i18n';
import { config } from '@/lib/config';
import { toMessage } from '@/components/ui/toast';

function CentredCard({
  icon: Icon,
  title,
  description,
  action,
}: {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex min-h-dvh items-center justify-center bg-background px-4">
      <div className="w-full max-w-md rounded-xl border border-border bg-surface p-8 text-center shadow-sm">
        <div className="mx-auto mb-4 w-fit rounded-full border border-border bg-surface-muted p-3 text-muted-foreground">
          <Icon className="size-5" />
        </div>
        <h1 className="text-base font-semibold">{title}</h1>
        <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{description}</p>
        {action ? <div className="mt-5">{action}</div> : null}
      </div>
    </div>
  );
}

/** Boot skeleton. Shows the shell silhouette rather than a blank page, so a
 *  slow identity provider does not look like a crash. */
function BootSkeleton() {
  return (
    <div className="grid min-h-dvh lg:grid-cols-[16rem_1fr]">
      <div className="hidden border-r border-border bg-surface p-3 lg:block">
        <Skeleton className="mb-6 h-8 w-40" />
        <div className="space-y-2">
          {Array.from({ length: 5 }, (_, index) => (
            <Skeleton key={index} className="h-8 w-full" />
          ))}
        </div>
      </div>
      <div className="p-6">
        <Skeleton className="mb-2 h-7 w-56" />
        <Skeleton className="mb-6 h-4 w-80" />
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className="h-24" />
          ))}
        </div>
      </div>
    </div>
  );
}

/**
 * Authentication and organisation gate.
 *
 * Reproduces the previous UI's `organizationRequired` blocker, which existed
 * because a Keycloak account with no organisation cannot use the platform,
 * but as a route guard rather than a hidden overlay div.
 */
export function AuthGate() {
  const t = useT();
  const { status, identity, error, login } = useAuth();

  if (status === 'initialising') return <BootSkeleton />;

  if (status === 'failed') {
    return (
      <CentredCard
        icon={ShieldAlert}
        title={t('errors.signedOutTitle')}
        description={toMessage(error)}
        action={
          <Button variant="primary" onClick={() => window.location.reload()}>
            {t('errors.reload')}
          </Button>
        }
      />
    );
  }

  if (status === 'anonymous') {
    return (
      <CentredCard
        icon={LogIn}
        title={config.brand.productName}
        description={t('errors.signedOutHint')}
        action={
          <Button variant="primary" onClick={login}>
            <LogIn aria-hidden />
            {t('nav.signIn')}
          </Button>
        }
      />
    );
  }

  // Organisations are only enforced when the deployment actually uses them,
  // so a CE install without Keycloak organisations is not locked out.
  const requiresOrganization = config.features['requireOrganization'] === true;
  if (requiresOrganization && identity && identity.organizations.length === 0) {
    return (
      <CentredCard
        icon={ShieldAlert}
        title={t('errors.noOrganizationTitle')}
        description={t('errors.noOrganizationHint')}
      />
    );
  }

  return <Outlet />;
}
