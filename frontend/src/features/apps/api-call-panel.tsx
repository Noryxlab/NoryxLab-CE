import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { KeyRound, Trash2 } from 'lucide-react';
import { CopyButton } from '@/components/common/copy-button';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardHeaderText,
  CardTitle,
} from '@/components/ui/card';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useToast } from '@/components/ui/toast';
import { projectTokensApi } from '@/lib/api/endpoints';
import { qk, useInvalidate, useProjectTokens } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import type { App } from '@/lib/api/types';

/**
 * How to call this endpoint, in the two forms people actually paste.
 *
 * Deploying is half the work; the half that decides whether anyone uses it is
 * knowing what to send and with which credential. Without this panel, a
 * developer has a URL and a vague idea that a token exists somewhere - which is
 * where an endpoint stops being used and starts being rebuilt elsewhere.
 *
 * The token is a project token: it outlives whoever created it, and it reaches
 * this project's endpoints and nothing else on the platform.
 */
export function ApiCallPanel({ api }: { api: App }) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const tokens = useProjectTokens(api.projectId);
  const [name, setName] = React.useState('');
  const [secret, setSecret] = React.useState<string | null>(null);

  const url = `${window.location.origin}${api.accessUrl || `/apis/${api.slug}`}`;
  // The secret only exists in this page, and only until it is reloaded. Before
  // it is created, the snippet shows the shape rather than a fake value.
  const shown = secret ?? 'NORYX_TOKEN';

  const curl = `curl -X POST ${url} \\
  -H "Authorization: Bearer ${shown}" \\
  -H "Content-Type: application/json" \\
  -d '{"input": "…"}'`;

  const python = `import os, requests

response = requests.post(
    "${url}",
    headers={"Authorization": f"Bearer {os.environ['NORYX_TOKEN']}"},
    json={"input": "…"},
    timeout=30,
)
response.raise_for_status()
print(response.json())`;

  const create = useMutation({
    mutationFn: () => projectTokensApi.create(api.projectId, { name: name.trim() }),
    onSuccess: (created) => {
      setSecret(created.secret);
      setName('');
      invalidate(qk.projectTokens(api.projectId));
    },
    onError: (error) => toast.error(error, t('apis.tokenCreate')),
  });

  const revoke = useMutation({
    mutationFn: (tokenId: string) => projectTokensApi.remove(api.projectId, tokenId),
    onSuccess: () => invalidate(qk.projectTokens(api.projectId)),
    onError: (error) => toast.error(error, t('apis.tokenRevoke')),
  });

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('apis.callTitle')}</CardTitle>
          <CardDescription>{t('apis.callHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        <Field label={t('apis.endpoint')}>
          <div className="flex items-center gap-2">
            <Input readOnly value={url} className="font-mono text-xs" />
            <CopyButton value={url} />
          </div>
        </Field>

        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium">curl</span>
            <CopyButton value={curl} />
          </div>
          <pre className="overflow-x-auto rounded-md border border-border bg-surface-muted p-3 text-xs">
            <code>{curl}</code>
          </pre>
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium">Python</span>
            <CopyButton value={python} />
          </div>
          <pre className="overflow-x-auto rounded-md border border-border bg-surface-muted p-3 text-xs">
            <code>{python}</code>
          </pre>
        </div>

        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (name.trim()) create.mutate();
          }}
          className="grid gap-2 sm:grid-cols-[1fr_auto] sm:items-end"
        >
          <Field label={t('apis.tokenName')} description={t('apis.tokenNameHint')}>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="scoring-prod"
            />
          </Field>
          <Button type="submit" variant="primary" loading={create.isPending} disabled={!name.trim()}>
            <KeyRound aria-hidden />
            {t('apis.tokenCreate')}
          </Button>
        </form>

        {/* Shown once, and said so plainly: a secret displayed a second time is
            a secret that was stored somewhere it should not have been. */}
        {secret ? (
          <div className="space-y-2 rounded-md border border-warning/40 bg-warning-subtle p-3">
            <p className="text-xs font-medium text-warning-foreground">{t('apis.tokenOnce')}</p>
            <div className="flex items-center gap-2">
              <Input readOnly value={secret} className="font-mono text-xs" />
              <CopyButton value={secret} />
            </div>
          </div>
        ) : null}

        {tokens.data?.length ? (
          <ul className="divide-y divide-border rounded-md border border-border">
            {tokens.data.map((token) => (
              <li key={token.id} className="flex items-center justify-between gap-3 px-3 py-2">
                <span className="min-w-0 text-xs">
                  <span className="font-medium">{token.name}</span>
                  <span className="ml-2 text-muted-foreground">
                    {token.lastUsedAt
                      ? t('apis.tokenLastUsed', { when: formatRelative(token.lastUsedAt, locale) })
                      : t('apis.tokenNeverUsed')}
                  </span>
                </span>
                <Button
                  variant="ghost"
                  size="sm"
                  loading={revoke.isPending}
                  onClick={() => revoke.mutate(token.id)}
                  aria-label={t('apis.tokenRevoke')}
                >
                  <Trash2 aria-hidden />
                </Button>
              </li>
            ))}
          </ul>
        ) : null}
      </CardContent>
    </Card>
  );
}
