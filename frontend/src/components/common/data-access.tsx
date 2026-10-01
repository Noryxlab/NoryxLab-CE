import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { useProjectExtracts } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';

export type DataAccess = 'dataset' | 'extracts';

/**
 * Ce que la charge pourra atteindre : les datasets entiers, ou seulement les
 * extraits.
 *
 * Le meme champ, les memes mots et le meme defaut pour un workspace, un job,
 * une tache planifiee et une application. La frontiere de l'ADR-038 n'existait
 * que pour le workspace, ce qui en faisait la propriete d'un ecran et non de
 * la plateforme : "l'equipe voit la selection et rien d'autre" tenait pendant
 * qu'on tapait et cessait des qu'un calcul tournait.
 *
 * Affiche seulement quand le projet a un extrait a monter. Proposer l'isolement
 * vers rien, c'est proposer une charge sans donnees - et le serveur le refuse,
 * donc l'offrir serait offrir un refus.
 */
export function DataAccessField({
  projectId,
  value,
  onChange,
}: {
  projectId: string;
  value: DataAccess;
  onChange: (value: DataAccess) => void;
}) {
  const t = useT();
  const extracts = useProjectExtracts(projectId);
  if ((extracts.data ?? []).length === 0) return null;

  return (
    <Field
      label={t('workspaces.dataAccessLabel')}
      description={
        value === 'extracts'
          ? t('workspaces.dataAccessExtractsHint')
          : t('workspaces.dataAccessDatasetHint')
      }
    >
      <Select
        value={value}
        onValueChange={(next) => onChange(next === 'extracts' ? 'extracts' : 'dataset')}
        options={[
          { value: 'dataset', label: t('workspaces.dataAccessDataset') },
          { value: 'extracts', label: t('workspaces.dataAccessExtracts') },
        ]}
      />
    </Field>
  );
}
