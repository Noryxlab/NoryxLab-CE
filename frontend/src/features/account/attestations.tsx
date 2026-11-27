import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { GraduationCap, Download, Trash2, Upload } from 'lucide-react';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardHeaderText,
  CardTitle,
} from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { SkeletonText } from '@/components/ui/skeleton';
import { ErrorState } from '@/components/common/states';
import { useToast } from '@/components/ui/toast';
import { useAttestations, qk, useInvalidate } from '@/lib/api/queries';
import { platformApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatDate } from '@/lib/format';
import type { Attestation } from '@/lib/api/types';

/**
 * Attestations de formation.
 *
 * Une personne depose le PDF qu'on lui a remis au bout d'une formation, pour
 * dire qu'elle sait manipuler de la donnee confidentielle. La plateforme
 * l'enregistre et **ne refuse rien** : ni un montage, ni un lancement, ni un
 * acces. On collecte avant d'exiger, parce qu'une installation qui se met a
 * exiger un champ que personne n'a rempli enferme tout le monde dehors le
 * premier jour.
 *
 * La date de fin est facultative et c'est pourtant elle qui compte : un
 * certificat qui expire, pas une case cochee. Sans date, le document dit
 * « cette personne a suivi la formation » ; avec, il dit « elle est couverte
 * aujourd'hui », et seule la deuxieme phrase sert a quelque chose.
 */
export function tonAttestationStatut(item: Attestation): { tone: 'success' | 'warning' | 'neutral'; key: string } {
  switch (item.status) {
    case 'valid':
      return { tone: 'success', key: 'account.attestationValid' };
    case 'expired':
      return { tone: 'warning', key: 'account.attestationExpired' };
    default:
      return { tone: 'neutral', key: 'account.attestationUndated' };
  }
}

export function AttestationsSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const attestations = useAttestations();
  const [subject, setSubject] = React.useState('');
  const [validUntil, setValidUntil] = React.useState('');
  const fileInput = React.useRef<HTMLInputElement>(null);

  const upload = useMutation({
    mutationFn: (file: File) =>
      platformApi.uploadAttestation({
        file,
        subject: subject.trim() || undefined,
        validUntil: validUntil || undefined,
      }),
    onSuccess: () => {
      invalidate(qk.attestations);
      setSubject('');
      setValidUntil('');
      if (fileInput.current) fileInput.current.value = '';
      toast.success(t('account.attestationUploaded'), t('account.attestations'));
    },
    onError: (error) => toast.error(error, t('account.attestations')),
  });

  const remove = useMutation({
    mutationFn: (id: string) => platformApi.deleteAttestation(id),
    onSuccess: () => invalidate(qk.attestations),
    onError: (error) => toast.error(error, t('common.delete')),
  });

  const items = attestations.data ?? [];

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle className="flex items-center gap-2">
              <GraduationCap className="size-5 text-muted-foreground" />
              {t('account.attestations')}
            </CardTitle>
            <CardDescription>{t('account.attestationsHint')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-end gap-2">
            <Field label={t('account.attestationSubject')} className="min-w-56">
              <Input
                value={subject}
                onChange={(event) => setSubject(event.target.value)}
                placeholder={t('account.attestationSubjectPlaceholder')}
                maxLength={160}
              />
            </Field>
            <Field
              label={t('account.attestationValidUntil')}
              className="min-w-44"
              description={t('account.attestationValidUntilHint')}
            >
              <Input type="date" value={validUntil} onChange={(event) => setValidUntil(event.target.value)} />
            </Field>
            <input
              ref={fileInput}
              type="file"
              accept="application/pdf"
              className="hidden"
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) upload.mutate(file);
              }}
            />
            <Button variant="primary" loading={upload.isPending} onClick={() => fileInput.current?.click()}>
              <Upload aria-hidden />
              {t('account.attestationUpload')}
            </Button>
          </div>

          {attestations.isLoading ? (
            <SkeletonText lines={2} />
          ) : attestations.isError ? (
            <ErrorState error={attestations.error} onRetry={() => void attestations.refetch()} />
          ) : items.length === 0 ? (
            // Un etat vide qui dit ce que ca changerait d'en deposer un, et
            // surtout ce que ca ne change pas : rien n'est bloque sans.
            <p className="text-sm text-muted-foreground">{t('account.attestationsEmpty')}</p>
          ) : (
            <ul className="space-y-2">
              {items.map((item) => {
                const status = tonAttestationStatut(item);
                return (
                  <li
                    key={item.id}
                    className="flex flex-wrap items-center gap-2 rounded-md border border-border px-3 py-2 text-sm"
                  >
                    <span className="font-medium">{item.subject || item.fileName}</span>
                    <Badge tone={status.tone}>{t(status.key as never)}</Badge>
                    {item.validUntil ? (
                      <span className="text-xs text-muted-foreground">
                        {t('account.attestationUntil', { date: formatDate(item.validUntil, locale) })}
                      </span>
                    ) : null}
                    <span className="text-xs text-muted-foreground">
                      {formatDate(item.uploadedAt, locale)} · {formatBytes(item.size, locale)}
                    </span>
                    <span className="ml-auto flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={t('common.download')}
                        onClick={() => void platformApi.downloadAttestation(item.id, item.fileName)}
                      >
                        <Download aria-hidden />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={t('common.delete')}
                        loading={remove.isPending}
                        onClick={() => remove.mutate(item.id)}
                      >
                        <Trash2 aria-hidden />
                      </Button>
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
