import { PageHeader } from '@/components/common/page-header';
import { ApiTokensSection } from '@/features/account/api-tokens';
import { SecretCatalog } from '@/features/catalog/secret-catalog';
import { useT } from '@/lib/i18n';

/**
 * The account page.
 *
 * Everything here belongs to the person looking at it, which is why it is not
 * in Administration. Api tokens went there first and were unreachable by the
 * people who need them: /admin is admin-only, and the researcher wiring up a CI
 * job is precisely not an administrator.
 *
 * Secrets are here for the same reason. They were only in the catalogue, which
 * is where datasets and environments live - shared, browsable, organisational.
 * A personal access token is none of those: it belongs to one person, and the
 * person looking for it goes to their account first. It stays in the catalogue
 * too, because that is where somebody attaching a secret to a repository is
 * already standing.
 */
export function AccountPage() {
  const t = useT();
  return (
    <div className="space-y-5">
      <PageHeader title={t('account.title')} description={t('account.subtitle')} />
      <ApiTokensSection />
      <SecretCatalog />
    </div>
  );
}
