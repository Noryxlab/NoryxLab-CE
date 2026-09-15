import * as React from 'react';
import { Plus, Save, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { SectionHeader } from '@/components/common/page-header';
import { useToast } from '@/components/ui/toast';
import { adminApi } from '@/lib/api/endpoints';
import { useRbacPolicy } from '@/lib/api/queries';
import { qk, useInvalidate } from '@/lib/api/queries';
import { useMutation } from '@tanstack/react-query';
import { useT } from '@/lib/i18n';
import type { RbacPolicyRow } from '@/lib/api/types';

/**
 * La ou une installation ecrit ses propres roles.
 *
 * Le document existait depuis des mois, le moteur le lit depuis peu, et
 * personne n'avait d'ecran pour l'ecrire : les lignes ne pouvaient etre
 * changees qu'en appelant l'API a la main. Un client qui decrit son
 * organisation - "data steward", "responsable qualite" - le fait ici.
 *
 * Deux choses ne se negocient pas sur cet ecran.
 *
 * Les lignes verrouillees decrivent ce que la plateforme fait deja. Elles sont
 * affichees parce qu'on ne peut pas raisonner sur ses propres roles sans voir
 * ceux d'en face, et elles ne s'editent pas : le serveur refuse, et une case
 * qui n'accepte rien ne doit pas avoir l'air modifiable.
 *
 * Et un role maison declare toujours de quel role de base il herite. C'est ce
 * que la personne obtient reellement partout ou la matrice n'a rien a dire -
 * donc le champ est a cote du nom, pas cache dans un repli.
 */

const PERMISSIONS = ['-', 'R', 'RW', 'Admin', 'R attaché', 'RW attaché'] as const;
const BASES = ['viewer', 'editor', 'admin'] as const;

const COLUMNS = [
  'project',
  'dataset',
  'ontology',
  'datasource',
  'environment',
  'workload',
  'governance',
] as const;

type ColumnKey = (typeof COLUMNS)[number];

export function RoleMatrixEditor() {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const policy = useRbacPolicy();

  // Le brouillon vit ici : on edite plusieurs cases avant d'enregistrer, et
  // sauver a chaque frappe enverrait un document incoherent a chaque lettre.
  const [draft, setDraft] = React.useState<RbacPolicyRow[] | null>(null);
  const rows = draft ?? policy.data?.rows ?? [];
  const counts = policy.data?.assignmentCounts ?? {};
  const dirty = draft !== null;

  const save = useMutation({
    mutationFn: (next: RbacPolicyRow[]) => adminApi.saveRbacPolicy(next),
    onSuccess: () => {
      invalidate(qk.adminRbacPolicy, qk.assignableRoles);
      setDraft(null);
      toast.success(t('roleMatrix.saved'), t('roleMatrix.title'));
    },
    onError: (error) => toast.error(error, t('roleMatrix.title')),
  });

  const update = (index: number, patch: Partial<RbacPolicyRow>) =>
    setDraft(rows.map((row, position) => (position === index ? { ...row, ...patch } : row)));

  const addRole = () =>
    setDraft([
      ...rows,
      {
        role: '',
        key: '',
        locked: false,
        basedOn: 'viewer',
        description: '',
        project: '-',
        dataset: '-',
        ontology: '-',
        datasource: '-',
        environment: '-',
        workload: '-',
        governance: '-',
      },
    ]);

  const removeRole = (index: number) => setDraft(rows.filter((_, position) => position !== index));

  const permissionOptions = PERMISSIONS.map((value) => ({ value, label: value }));
  const baseOptions = BASES.map((value) => ({
    value,
    label: t(`roleMatrix.base${capitalise(value)}` as never),
  }));

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('roleMatrix.title')}
        description={t('roleMatrix.intro')}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={addRole}>
              <Plus className="size-4" aria-hidden />
              {t('roleMatrix.addRole')}
            </Button>
            <Button
              size="sm"
              disabled={!dirty || save.isPending}
              onClick={() => save.mutate(rows)}
            >
              <Save className="size-4" aria-hidden />
              {t('common.save')}
            </Button>
          </div>
        }
      />

      <Card className="overflow-x-auto p-0">
        <table className="w-full min-w-[56rem] border-collapse text-sm">
          <thead>
            <tr className="border-b border-border text-left">
              <th className="px-4 py-2 font-medium">{t('common.role')}</th>
              <th className="px-3 py-2 font-medium">{t('roleMatrix.basedOn')}</th>
              {COLUMNS.map((column) => (
                <th key={column} className="px-2 py-2 font-medium">
                  {t(`roleMatrix.column${capitalise(column)}` as never)}
                </th>
              ))}
              <th className="px-2 py-2" />
            </tr>
          </thead>
          <tbody>
            {rows.map((row, index) => {
              const held = counts[row.key] ?? 0;
              return (
                <tr key={row.key || `new-${index}`} className="border-b border-border/60 align-top">
                  <td className="px-4 py-3">
                    {row.locked ? (
                      <div className="min-w-0">
                        <p className="font-medium">{row.role}</p>
                        <p className="max-w-xs text-xs text-muted-foreground">{row.description}</p>
                      </div>
                    ) : (
                      <div className="min-w-0 space-y-1.5">
                        <Input
                          value={row.role}
                          onChange={(event) => update(index, { role: event.target.value })}
                          placeholder={t('roleMatrix.namePlaceholder')}
                          className="w-48"
                        />
                        <Input
                          value={row.description ?? ''}
                          onChange={(event) => update(index, { description: event.target.value })}
                          placeholder={t('roleMatrix.descriptionPlaceholder')}
                          className="w-64 text-xs"
                        />
                      </div>
                    )}
                    <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                      {row.locked ? <Badge tone="neutral">{t('roleMatrix.shipped')}</Badge> : null}
                      {/* Un role porte ne peut pas etre supprime, et le nombre
                          dit par combien de personnes. Sans lui, la suppression
                          echoue et l'administrateur ne sait pas pourquoi. */}
                      {held > 0 ? (
                        <span className="text-xs text-muted-foreground">
                          {t('roleMatrix.heldBy').replace('{count}', String(held))}
                        </span>
                      ) : null}
                    </div>
                  </td>
                  <td className="px-3 py-3">
                    {row.locked ? (
                      <span className="text-xs text-muted-foreground">
                        {t('roleMatrix.shippedBase')}
                      </span>
                    ) : (
                      <Select
                        value={row.basedOn ?? 'viewer'}
                        onValueChange={(value) => update(index, { basedOn: value })}
                        options={baseOptions}
                        className="w-36"
                      />
                    )}
                  </td>
                  {COLUMNS.map((column) => (
                    <td key={column} className="px-2 py-3">
                      {row.locked ? (
                        <Badge tone={row[column] === '-' ? 'outline' : 'neutral'}>{row[column]}</Badge>
                      ) : (
                        <Select
                          value={row[column as ColumnKey]}
                          onValueChange={(value) => update(index, { [column]: value } as Partial<RbacPolicyRow>)}
                          options={permissionOptions}
                          className="w-28"
                        />
                      )}
                    </td>
                  ))}
                  <td className="px-2 py-3">
                    {row.locked ? null : (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => removeRole(index)}
                        title={t('common.delete')}
                      >
                        <Trash2 className="size-4" aria-hidden />
                        <span className="sr-only">{t('common.delete')}</span>
                      </Button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </Card>

      {/* Dit une fois, sous le tableau : c'est la regle qui explique la
          colonne "Herite de", et la repeter sur chaque ligne ferait du bruit. */}
      <p className="max-w-prose text-sm text-muted-foreground">{t('roleMatrix.ceilingHint')}</p>
    </div>
  );
}

function capitalise(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
