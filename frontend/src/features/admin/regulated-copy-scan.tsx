import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Fingerprint, ShieldAlert, ShieldCheck } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useToast } from '@/components/ui/toast';
import { adminApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatNumber } from '@/lib/format';
import type { RegulatedCopyScanReport } from '@/lib/api/types';

/** Le croisement d'empreintes d'ADR-060, declenche a la main.
 *
 *  Il tourne chaque nuit ; ce bouton existe parce qu'un administrateur qui
 *  vient d'accorder « ecrire a cote d'une donnee reglementee » ne peut pas
 *  attendre vingt-quatre heures pour savoir ce que ca a produit. La passe lit
 *  des listages, pas des octets.
 */
export function RegulatedCopyScanCard() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const [report, setReport] = React.useState<RegulatedCopyScanReport | null>(null);

  const run = useMutation({
    mutationFn: () => adminApi.runRegulatedCopyScan(),
    onSuccess: (result) => {
      setReport(result);
      if (result.findings.length > 0) {
        toast.warning(t('admin.copyScanFound'), t('admin.copyScan'));
        return;
      }
      toast.success(t('admin.copyScanClean'), t('admin.copyScan'));
    },
    onError: (error) => toast.error(error, t('admin.copyScan')),
  });

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('admin.copyScan')}</CardTitle>
          <p className="text-sm text-muted-foreground">{t('admin.copyScanHint')}</p>
        </CardHeaderText>
        <Button variant="secondary" loading={run.isPending} onClick={() => run.mutate()}>
          <Fingerprint aria-hidden />
          {t('admin.copyScanRun')}
        </Button>
      </CardHeader>

      {report ? (
        <CardContent className="space-y-3">
          <p className="text-sm text-muted-foreground">
            {t('admin.copyScanPairs', { count: formatNumber(report.pairsCompared, locale) })}
            {report.pairsSkipped > 0
              ? ` · ${t('admin.copyScanSkipped', { count: formatNumber(report.pairsSkipped, locale) })}`
              : ''}
          </p>

          {report.findings.length === 0 ? (
            // Un etat vide qui dit pourquoi il est vide : zero paire comparee
            // et zero copie trouvee ne veulent pas dire la meme chose.
            <div className="flex items-start gap-2 rounded-md border border-border bg-muted/40 p-3 text-sm">
              <ShieldCheck className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
              <p>
                {report.pairsCompared === 0
                  ? t('admin.copyScanNoPairs')
                  : t('admin.copyScanClean')}
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {report.findings.map((finding) => (
                <div
                  key={`${finding.regulatedDatasetId}:${finding.openDatasetId}`}
                  className="space-y-2 rounded-md border border-destructive/40 bg-destructive/5 p-3"
                >
                  <div className="flex flex-wrap items-center gap-2 text-sm">
                    <ShieldAlert className="size-4 shrink-0 text-destructive" aria-hidden />
                    <span className="font-medium">{finding.regulatedDatasetName}</span>
                    <span className="text-muted-foreground">→</span>
                    <span className="font-medium">{finding.openDatasetName}</span>
                    <Badge tone="danger">
                      {t('admin.copyScanIdentical', {
                        count: formatNumber(finding.identical, locale),
                      })}
                    </Badge>
                    <Badge tone="warning">
                      {t('admin.copyScanProbable', {
                        count: formatNumber(finding.probable, locale),
                      })}
                    </Badge>
                    <span className="text-xs text-muted-foreground">
                      {formatBytes(finding.totalSize, locale)}
                    </span>
                  </div>
                  <div className="overflow-x-auto">
                    <table className="w-full text-xs">
                      <tbody>
                        {finding.copies.map((copy) => (
                          <tr key={`${copy.regulatedPath}:${copy.openPath}`} className="align-top">
                            <td className="py-0.5 pr-3 font-mono">{copy.regulatedPath}</td>
                            <td className="py-0.5 pr-3 font-mono">{copy.openPath}</td>
                            <td className="py-0.5 pr-3 text-muted-foreground">
                              {formatBytes(copy.size, locale)}
                            </td>
                            <td className="py-0.5 text-muted-foreground">
                              {copy.confidence === 'identical'
                                ? t('admin.copyScanConfidenceIdentical')
                                : t('admin.copyScanConfidenceProbable')}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Ce que le croisement ne voit pas, dit a l'endroit ou on lit son
              resultat : sans cette phrase, « rien trouve » se lit comme
              « rien n'est sorti ». */}
          <p className="text-xs text-muted-foreground">{t('admin.copyScanLimits')}</p>
        </CardContent>
      ) : null}
    </Card>
  );
}
