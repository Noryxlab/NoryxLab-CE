import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Mail, Send } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardFooter, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import { adminApi } from '@/lib/api/endpoints';
import { qk, useInvalidate, useSmtp } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import type { SmtpSettings } from '@/lib/api/types';

/**
 * The mail server, filled in per installation.
 *
 * Essilor sends through their relay, EMSE through theirs, a demo platform
 * through whatever it has - so this is a form rather than a provider chosen in
 * a manifest. Keycloak holds the values, because Keycloak is what sends the
 * mail; storing a copy here would create a second truth and the one that
 * matters would be the one nobody edited.
 *
 * The password is write-only. It is never sent back to a browser, so an
 * administrator cannot read someone else's credential out of this screen - the
 * field shows whether one is stored, and an empty field keeps it.
 */

const BLANK: SmtpSettings = {
  host: '',
  port: '587',
  from: '',
  fromDisplayName: '',
  replyTo: '',
  user: '',
  auth: true,
  starttls: true,
  ssl: false,
  passwordSet: false,
};

export function SmtpSettingsSection() {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const smtp = useSmtp();

  const [form, setForm] = React.useState<SmtpSettings>(BLANK);
  const [password, setPassword] = React.useState('');
  const [recipient, setRecipient] = React.useState('');

  React.useEffect(() => {
    if (smtp.data?.settings) setForm({ ...BLANK, ...smtp.data.settings });
  }, [smtp.data]);

  const set = <K extends keyof SmtpSettings>(key: K, value: SmtpSettings[K]) =>
    setForm((current) => ({ ...current, [key]: value }));

  const save = useMutation({
    mutationFn: () => adminApi.updateSmtp({ ...form, password: password || undefined }),
    onSuccess: () => {
      setPassword('');
      invalidate(qk.adminSmtp);
      toast.success(t('common.save'), t('admin.smtp'));
    },
    onError: (error) => toast.error(error, t('admin.smtp')),
  });

  // Testing before saving, deliberately: an administrator can prove a change
  // works without first replacing a configuration that does.
  const test = useMutation({
    mutationFn: () =>
      adminApi.testSmtp({ ...form, password: password || undefined, testRecipient: recipient.trim() }),
    onSuccess: (result) => toast.success(t('admin.smtpTestSent', { recipient: result.recipient }), t('admin.smtp')),
    onError: (error) => toast.error(error, t('admin.smtpTest')),
  });

  if (smtp.isLoading) return <Skeleton className="h-64 w-full" />;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle className="flex items-center gap-2">
            <Mail className="size-4" aria-hidden />
            {t('admin.smtp')}
            {smtp.data?.configured ? (
              <Badge tone="success">{t('admin.smtpConfigured')}</Badge>
            ) : (
              <Badge tone="warning">{t('admin.smtpMissing')}</Badge>
            )}
          </CardTitle>
          <CardDescription>{t('admin.smtpHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>

      <CardContent className="grid gap-4 sm:grid-cols-2">
        <Field label={t('admin.smtpHost')} required>
          <Input value={form.host} onChange={(event) => set('host', event.target.value)} placeholder="smtp.example.org" />
        </Field>
        <Field label={t('admin.smtpPort')}>
          <Input value={form.port} onChange={(event) => set('port', event.target.value)} placeholder="587" />
        </Field>
        <Field label={t('admin.smtpFrom')} description={t('admin.smtpFromHint')} required>
          <Input value={form.from} onChange={(event) => set('from', event.target.value)} placeholder="noreply@example.org" />
        </Field>
        <Field label={t('admin.smtpFromName')}>
          <Input value={form.fromDisplayName} onChange={(event) => set('fromDisplayName', event.target.value)} />
        </Field>
        <Field label={t('admin.smtpReplyTo')}>
          <Input value={form.replyTo} onChange={(event) => set('replyTo', event.target.value)} />
        </Field>
        <Field label={t('admin.smtpUser')} description={form.auth ? undefined : t('admin.smtpNoAuthHint')}>
          <Input value={form.user} onChange={(event) => set('user', event.target.value)} disabled={!form.auth} />
        </Field>
        <Field
          label={t('admin.smtpPassword')}
          description={form.passwordSet ? t('admin.smtpPasswordStored') : t('admin.smtpPasswordHint')}
        >
          <Input
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            placeholder={form.passwordSet ? '••••••••' : ''}
            disabled={!form.auth}
            autoComplete="new-password"
          />
        </Field>

        <div className="flex flex-col justify-end gap-2 text-sm">
          <label className="flex items-center gap-2">
            <input type="checkbox" checked={form.auth} onChange={(event) => set('auth', event.target.checked)} className="size-4 accent-[var(--noryx-brand)]" />
            {t('admin.smtpAuth')}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={form.starttls}
              onChange={(event) => {
                set('starttls', event.target.checked);
                // STARTTLS upgrades a plain connection; SSL wraps it from the
                // start. Both at once is a configuration no server implements.
                if (event.target.checked) set('ssl', false);
              }}
              className="size-4 accent-[var(--noryx-brand)]"
            />
            STARTTLS
          </label>
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={form.ssl}
              onChange={(event) => {
                set('ssl', event.target.checked);
                if (event.target.checked) set('starttls', false);
              }}
              className="size-4 accent-[var(--noryx-brand)]"
            />
            SSL/TLS
          </label>
        </div>
      </CardContent>

      <CardFooter className="flex-wrap items-end justify-between gap-3">
        <div className="flex items-end gap-2">
          <Field label={t('admin.smtpTestTo')} description={t('admin.smtpTestHint')}>
            <Input
              value={recipient}
              onChange={(event) => setRecipient(event.target.value)}
              placeholder="vous@example.org"
              type="email"
            />
          </Field>
          <Button
            variant="secondary"
            loading={test.isPending}
            disabled={!form.host.trim() || !recipient.trim()}
            onClick={() => test.mutate()}
          >
            <Send className="size-4" aria-hidden />
            {t('admin.smtpTest')}
          </Button>
        </div>
        <Button variant="primary" loading={save.isPending} onClick={() => save.mutate()}>
          {t('common.save')}
        </Button>
      </CardFooter>
    </Card>
  );
}
