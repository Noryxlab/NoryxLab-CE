import * as React from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Trash2, UserX } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { Sheet, SheetBody, SheetContent, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { useToast } from '@/components/ui/toast';
import { adminApi } from '@/lib/api/endpoints';
import { qk, useAdminUsers, useInvalidate } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import type { PlatformUser } from '@/lib/api/types';

/**
 * Disabling an account, with the handover it requires.
 *
 * The screen asks for a successor *before* the administrator commits, because
 * the alternative is a refusal after the fact: the platform knows what the
 * account owns, so it says so up front and names the resources rather than
 * counting them. Somebody deciding who inherits a project needs to know which
 * project.
 *
 * The consequences that are easy to forget are stated on the screen and not
 * only in the API's answer: the account's API tokens are revoked, and its
 * personal secrets stay with it.
 *
 * The same sheet removes an account for good, and only once it is already
 * disabled - which is why the two live together. Deletion asks the same
 * question about ownership, warns that it cannot be undone, and says the one
 * thing that differs: personal secrets are deleted rather than left behind,
 * because there is no longer an account to leave them with.
 */
export function DeactivateUserSheet({
  user,
  open,
  onOpenChange,
}: {
  user: PlatformUser | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const users = useAdminUsers();
  const [successor, setSuccessor] = React.useState('');
  // Disabling comes first and deletion second, so the account's own state
  // decides which of the two this sheet is: an enabled account can only be
  // disabled, and the API refuses to delete one anyway.
  const removing = user?.enabled === false;

  const owned = useQuery({
    queryKey: ['admin', 'owned', user?.id ?? ''],
    queryFn: () => adminApi.ownedBy(user?.id ?? ''),
    enabled: open && Boolean(user?.id),
  });

  const items = React.useMemo(() => {
    const owns = owned.data?.owns ?? {};
    return [
      { label: t('admin.ownedProjects'), values: owns.projects ?? [] },
      { label: t('admin.ownedDatasets'), values: owns.datasets ?? [] },
      { label: t('admin.ownedOntologies'), values: owns.ontologies ?? [] },
      { label: t('admin.ownedApps'), values: owns.apps ?? [] },
      { label: t('admin.ownedDatasources'), values: owns.datasources ?? [] },
      { label: t('admin.ownedRepositories'), values: owns.repositories ?? [] },
    ].filter((group) => group.values.length > 0);
  }, [owned.data, t]);

  const ownsSomething = (owned.data?.count ?? 0) > 0;

  const deactivate = useMutation({
    mutationFn: () => adminApi.deactivateUser(user?.id ?? '', successor),
    onSuccess: (result) => {
      invalidate(qk.adminUsers);
      onOpenChange(false);
      setSuccessor('');
      toast.success(
        t('admin.deactivatedHint', { count: String(result.tokensRevoked) }),
        t('admin.deactivate'),
      );
    },
    onError: (error) => toast.error(error, t('admin.deactivate')),
  });

  const remove = useMutation({
    mutationFn: () => adminApi.deleteUser(user?.id ?? '', successor),
    onSuccess: (result) => {
      invalidate(qk.adminUsers);
      onOpenChange(false);
      setSuccessor('');
      toast.success(
        t('admin.deletedHint', { count: String(result.secretsDeleted) }),
        t('admin.deleteAccount'),
      );
    },
    onError: (error) => toast.error(error, t('admin.deleteAccount')),
  });

  const candidates = (users.data ?? [])
    .filter((candidate) => candidate.id !== user?.id && candidate.enabled !== false)
    .map((candidate) => ({
      value: candidate.username ?? candidate.id,
      label: candidate.username ?? candidate.id,
      hint: candidate.email ?? undefined,
    }));

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent aria-describedby={undefined}>
        <SheetHeader>
          <SheetTitle>
            {removing ? t('admin.deleteAccount') : t('admin.deactivate')} ·{' '}
            {user?.username ?? user?.id}
          </SheetTitle>
        </SheetHeader>
        <SheetBody className="space-y-4">
          <p className="text-sm text-muted-foreground">
            {removing ? t('admin.deleteHint') : t('admin.deactivateHint')}
          </p>

          {owned.isLoading ? (
            <p className="text-sm text-muted-foreground">{t('common.loading')}</p>
          ) : ownsSomething ? (
            <>
              <div className="rounded-md border border-border p-3 text-sm">
                <p className="mb-2 font-medium">{t('admin.ownedTitle')}</p>
                {items.map((group) => (
                  <p key={group.label} className="text-xs text-muted-foreground">
                    <span className="font-medium text-foreground">{group.label}</span>{' '}
                    {group.values.join(', ')}
                  </p>
                ))}
              </div>
              <Field label={t('admin.successorLabel')} description={t('admin.successorHint')} required>
                <Select value={successor} onValueChange={setSuccessor} options={candidates} />
              </Field>
            </>
          ) : (
            <p className="text-sm text-muted-foreground">{t('admin.ownsNothing')}</p>
          )}

          {/* Said on the screen, not only in the API's answer: both are easy to
              forget and neither is reversible by re-enabling the account. */}
          <ul className="list-disc space-y-1 pl-5 text-xs text-muted-foreground">
            <li>{t('admin.deactivateTokens')}</li>
            <li>{removing ? t('admin.deleteSecrets') : t('admin.deactivateSecrets')}</li>
            {removing ? <li>{t('admin.deleteAudit')}</li> : null}
          </ul>
        </SheetBody>
        <SheetFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          {removing ? (
            <Button
              variant="danger"
              loading={remove.isPending}
              disabled={ownsSomething && !successor}
              onClick={() => remove.mutate()}
            >
              <Trash2 aria-hidden />
              {t('admin.deleteAccount')}
            </Button>
          ) : (
            <Button
              variant="danger-outline"
              loading={deactivate.isPending}
              disabled={ownsSomething && !successor}
              onClick={() => deactivate.mutate()}
            >
              <UserX aria-hidden />
              {t('admin.deactivate')}
            </Button>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
