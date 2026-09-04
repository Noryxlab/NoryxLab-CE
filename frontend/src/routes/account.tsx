import { PageHeader } from '@/components/common/page-header';
import { ApiTokensSection } from '@/features/account/api-tokens';
import { useT } from '@/lib/i18n';

/**
 * The account page.
 *
 * Everything here belongs to the person looking at it, which is why it is not
 * in Administration. Api tokens went there first and were unreachable by the
 * people who need them: /admin is admin-only, and the researcher wiring up a CI
 * job is precisely not an administrator.
 */
export function AccountPage() {
  const t = useT();
  return (
    <div className="space-y-5">
      <PageHeader title={t('account.title')} description={t('account.subtitle')} />
      <ApiTokensSection />
    </div>
  );
}
