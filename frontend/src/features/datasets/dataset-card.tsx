import * as React from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input, Textarea } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { datasetCardApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/api/queries';
import type { Dataset, DatasetCardCheck, DatasetCardContent, DatasetCardVerdict } from '@/lib/api/types';
import { useT, type TranslationKey } from '@/lib/i18n';

/** Ce qu'un dataset dit de lui-meme, et ce que la plateforme en verifie
 *  (ADR-047).
 *
 *  Le scan produit un inventaire de chemins — sujets, visites, modalites,
 *  comptes, volumes — et le dit honnetement. Ce qu'il ne peut pas produire,
 *  c'est ce dont le dataset *parle* : sa finalite, ce pour quoi il ne doit pas
 *  servir, la base legale, les unites, a qui demander. Rien de tout ca ne se
 *  deduit d'un octet, et c'est la plus grande moitie de ce qui manquait.
 *
 *  L'ecran a une regle : une declaration confirmee et une declaration que
 *  personne n'a verifiee ne doivent pas se ressembler. C'est tout l'interet de
 *  la colonne de droite, et c'est pour ca que `unverified` porte un ton
 *  d'avertissement plutot qu'un ton neutre — un lecteur presse lit les couleurs
 *  avant les mots. */

const TONES: Record<DatasetCardVerdict, 'success' | 'danger' | 'warning' | 'neutral' | 'outline'> = {
  agrees: 'success',
  differs: 'danger',
  unverified: 'warning',
  undeclared: 'neutral',
  not_checkable: 'outline',
};

const VERDICT_LABELS: Record<DatasetCardVerdict, TranslationKey> = {
  agrees: 'datasetCard.verdictAgrees',
  differs: 'datasetCard.verdictDiffers',
  unverified: 'datasetCard.verdictUnverified',
  undeclared: 'datasetCard.verdictUndeclared',
  not_checkable: 'datasetCard.verdictNotCheckable',
};

const FIELD_LABELS: Record<string, TranslationKey> = {
  subjects: 'datasetCard.subjects',
  objects: 'datasetCard.objects',
  modalities: 'datasetCard.modalities',
  firstVisit: 'datasetCard.firstVisit',
  lastVisit: 'datasetCard.lastVisit',
  pseudonymised: 'datasetCard.pseudonymised',
  purpose: 'datasetCard.purpose',
  outOfScope: 'datasetCard.outOfScope',
  limitations: 'datasetCard.limitations',
  legalBasis: 'datasetCard.legalBasis',
  consent: 'datasetCard.consent',
  licence: 'datasetCard.licence',
};

/** Les champs libres, dans l'ordre ou on les lit : ce que c'est, pour quoi,
 *  d'ou ca vient, comment le lire, a qui demander. */
const PROSE: { key: keyof DatasetCardContent; label: TranslationKey; long?: boolean }[] = [
  { key: 'study', label: 'datasetCard.study' },
  { key: 'release', label: 'datasetCard.release' },
  { key: 'population', label: 'datasetCard.population', long: true },
  { key: 'inclusion', label: 'datasetCard.inclusion', long: true },
  { key: 'purpose', label: 'datasetCard.purpose', long: true },
  { key: 'outOfScope', label: 'datasetCard.outOfScope', long: true },
  { key: 'limitations', label: 'datasetCard.limitations', long: true },
  { key: 'provenance', label: 'datasetCard.provenance', long: true },
  { key: 'legalBasis', label: 'datasetCard.legalBasis' },
  { key: 'consent', label: 'datasetCard.consent' },
  { key: 'licence', label: 'datasetCard.licence' },
  { key: 'units', label: 'datasetCard.units' },
  { key: 'conventions', label: 'datasetCard.conventions', long: true },
  { key: 'contact', label: 'datasetCard.contact' },
];

/** Un champ que la table ne connait pas s'affiche tel quel : un libelle
 *  manquant ne doit pas faire disparaitre la ligne de controle qui le porte. */
function labelFor(t: (key: TranslationKey) => string, field: string): string {
  const key = FIELD_LABELS[field];
  return key ? t(key) : field;
}

type Draft = Record<string, string>;

function draftFrom(card: DatasetCardContent | null | undefined): Draft {
  const draft: Draft = {};
  for (const { key } of PROSE) draft[key] = String(card?.[key] ?? '');
  draft.subjects = card?.claims?.subjects != null ? String(card.claims.subjects) : '';
  draft.objects = card?.claims?.objects != null ? String(card.claims.objects) : '';
  draft.modalities = (card?.claims?.modalities ?? []).join(', ');
  draft.firstVisit = card?.claims?.firstVisit ?? '';
  draft.lastVisit = card?.claims?.lastVisit ?? '';
  draft.pseudonymised =
    card?.claims?.pseudonymised == null ? '' : card.claims.pseudonymised ? 'true' : 'false';
  return draft;
}

/** Vide veut dire "rien declare", pas "zero". Envoyer 0 pour un champ qu'on
 *  n'a pas rempli ferait apparaitre un ecart la ou il n'y a qu'un silence. */
function numberOrNull(value: string): number | null {
  const trimmed = value.trim();
  if (trimmed === '') return null;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : null;
}

function payloadFrom(draft: Draft): Record<string, unknown> {
  const payload: Record<string, unknown> = {};
  for (const { key } of PROSE) payload[key] = draft[key]?.trim() ?? '';
  payload.subjects = numberOrNull(draft.subjects ?? '');
  payload.objects = numberOrNull(draft.objects ?? '');
  payload.modalities = (draft.modalities ?? '')
    .split(',')
    .map((name) => name.trim())
    .filter(Boolean);
  payload.firstVisit = (draft.firstVisit ?? '').trim();
  payload.lastVisit = (draft.lastVisit ?? '').trim();
  payload.pseudonymised =
    draft.pseudonymised === 'true' ? true : draft.pseudonymised === 'false' ? false : null;
  return payload;
}

export function DatasetCardPanel({ dataset }: { dataset: Dataset }) {
  const t = useT();
  const client = useQueryClient();
  const [editing, setEditing] = React.useState(false);
  const [draft, setDraft] = React.useState<Draft>({});

  const card = useQuery({
    queryKey: qk.datasetCard(dataset.id),
    queryFn: () => datasetCardApi.get(dataset.id),
    enabled: Boolean(dataset.id),
  });

  const save = useMutation({
    mutationFn: () => datasetCardApi.set(dataset.id, payloadFrom(draft)),
    onSuccess: () => {
      setEditing(false);
      void client.invalidateQueries({ queryKey: qk.datasetCard(dataset.id) });
    },
  });

  const content = card.data?.card ?? null;
  const checks = card.data?.checks ?? [];

  function startEditing() {
    setDraft(draftFrom(content));
    setEditing(true);
  }

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('datasetCard.title')}</CardTitle>
          <CardDescription>{t('datasetCard.hint')}</CardDescription>
        </CardHeaderText>
        {!editing ? (
          <Button variant="secondary" onClick={startEditing} disabled={card.isLoading}>
            {card.data?.declared ? t('datasetCard.edit') : t('datasetCard.declare')}
          </Button>
        ) : null}
      </CardHeader>

      <CardContent className="space-y-5">
        {editing ? (
          <Editor
            draft={draft}
            onChange={(key, value) => setDraft((current) => ({ ...current, [key]: value }))}
            onCancel={() => setEditing(false)}
            onSave={() => save.mutate()}
            saving={save.isPending}
          />
        ) : (
          <>
            {/* Vide est un etat legitime et se montre comme vide : un champ sans
                reponse est une information, et le remplir d'une supposition est
                precisement ce qu'on veut eviter. */}
            {card.data?.declared ? (
              <Declared content={content} />
            ) : (
              <p className="text-sm text-muted-foreground">{t('datasetCard.undeclaredAll')}</p>
            )}

            <Separator />
            <Control
              checks={checks}
              measuredBy={card.data?.measuredBy}
              measuredAt={card.data?.measuredAt}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

function Declared({ content }: { content: DatasetCardContent | null }) {
  const t = useT();
  if (!content) return null;
  const filled = PROSE.filter(({ key }) => String(content[key] ?? '').trim() !== '');
  return (
    <div className="space-y-4">
      <dl className="grid gap-3 sm:grid-cols-2">
        {filled.map(({ key, label }) => (
          <div key={key} className="space-y-1">
            <dt className="text-xs font-medium text-muted-foreground">{t(label)}</dt>
            <dd className="text-sm whitespace-pre-line">{String(content[key])}</dd>
          </div>
        ))}
      </dl>
      <p className="text-xs text-muted-foreground">
        {t('datasetCard.versionLine', {
          version: String(content.version),
          who: content.declaredBy ?? '—',
        })}
      </p>
    </div>
  );
}

/** La colonne de controle. La confiance n'exclut pas le controle : chaque
 *  figure declaree que la plateforme sait mesurer est confrontee, et le verdict
 *  se place *a cote* de la declaration — jamais par-dessus. Remplacer la parole
 *  de quelqu'un par une mesure detruit la seule preuve qu'ils etaient en
 *  desaccord, et c'est ca le fait interessant. */
function Control({
  checks,
  measuredBy,
  measuredAt,
}: {
  checks: DatasetCardCheck[];
  measuredBy?: string;
  measuredAt?: string;
}) {
  const t = useT();
  if (checks.length === 0) return null;

  // Ce qui demande une action en haut : un ecart, puis une declaration que rien
  // n'a verifiee. Le reste suit.
  const ordre: DatasetCardVerdict[] = ['differs', 'unverified', 'agrees', 'undeclared', 'not_checkable'];
  const tries = [...checks].sort((a, b) => ordre.indexOf(a.verdict) - ordre.indexOf(b.verdict));

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h4 className="text-sm font-medium">{t('datasetCard.control')}</h4>
        {measuredBy ? (
          <p className="text-xs text-muted-foreground">
            {t('datasetCard.measuredBy', {
              method: measuredBy,
              when: measuredAt ? new Date(measuredAt).toLocaleString() : '—',
            })}
          </p>
        ) : (
          <p className="text-xs text-muted-foreground">{t('datasetCard.neverMeasured')}</p>
        )}
      </div>

      <ul className="divide-y divide-border">
        {tries.map((check) => (
          <li key={check.field} className="flex flex-wrap items-baseline gap-x-3 gap-y-1 py-2">
            <span className="min-w-32 text-sm">{labelFor(t, check.field)}</span>
            <Badge tone={TONES[check.verdict]}>{t(VERDICT_LABELS[check.verdict])}</Badge>
            {check.declared ? (
              <span className="text-xs text-muted-foreground">
                {t('datasetCard.declaredValue')} <span className="font-mono">{check.declared}</span>
              </span>
            ) : null}
            {check.measured ? (
              <span className="text-xs text-muted-foreground">
                {t('datasetCard.measuredValue')} <span className="font-mono">{check.measured}</span>
              </span>
            ) : null}
            {check.note ? (
              <span className="basis-full text-xs text-muted-foreground">{check.note}</span>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}

function Editor({
  draft,
  onChange,
  onCancel,
  onSave,
  saving,
}: {
  draft: Draft;
  onChange: (key: string, value: string) => void;
  onCancel: () => void;
  onSave: () => void;
  saving: boolean;
}) {
  const t = useT();
  return (
    <div className="space-y-5">
      <div className="grid gap-3 sm:grid-cols-2">
        {PROSE.map(({ key, label, long }) => (
          <Field key={key} label={t(label)} className={long ? 'sm:col-span-2' : undefined}>
            {long ? (
              <Textarea
                value={draft[key] ?? ''}
                onChange={(event) => onChange(key, event.target.value)}
              />
            ) : (
              <Input
                value={draft[key] ?? ''}
                onChange={(event) => onChange(key, event.target.value)}
              />
            )}
          </Field>
        ))}
      </div>

      <Separator />

      {/* Les figures verifiables, separees de la prose parce que la prose ne se
          verifie pas et que celles-ci le sont. */}
      <div className="space-y-2">
        <h4 className="text-sm font-medium">{t('datasetCard.claims')}</h4>
        <p className="text-xs text-muted-foreground">{t('datasetCard.claimsHint')}</p>
        <div className="grid gap-3 sm:grid-cols-3">
          <Field label={t('datasetCard.subjects')}>
            <Input
              inputMode="numeric"
              value={draft.subjects ?? ''}
              onChange={(event) => onChange('subjects', event.target.value)}
            />
          </Field>
          <Field label={t('datasetCard.objects')}>
            <Input
              inputMode="numeric"
              value={draft.objects ?? ''}
              onChange={(event) => onChange('objects', event.target.value)}
            />
          </Field>
          <Field
            label={t('datasetCard.pseudonymised')}
            description={t('datasetCard.pseudonymisedHint')}
          >
            <Select
              value={draft.pseudonymised ?? ''}
              onValueChange={(value) => onChange('pseudonymised', value)}
              placeholder={t('datasetCard.notStated')}
              options={[
                { value: 'true', label: t('common.yes') },
                { value: 'false', label: t('common.no') },
              ]}
            />
          </Field>
          <Field label={t('datasetCard.modalities')} className="sm:col-span-3">
            <Input
              value={draft.modalities ?? ''}
              onChange={(event) => onChange('modalities', event.target.value)}
              placeholder="DICOM, OCT"
            />
          </Field>
          <Field label={t('datasetCard.firstVisit')}>
            <Input
              value={draft.firstVisit ?? ''}
              onChange={(event) => onChange('firstVisit', event.target.value)}
              placeholder="AAAAMMJJ"
            />
          </Field>
          <Field label={t('datasetCard.lastVisit')}>
            <Input
              value={draft.lastVisit ?? ''}
              onChange={(event) => onChange('lastVisit', event.target.value)}
              placeholder="AAAAMMJJ"
            />
          </Field>
        </div>
      </div>

      <div className="flex gap-2">
        <Button variant="primary" loading={saving} onClick={onSave}>
          {t('common.save')}
        </Button>
        <Button variant="ghost" onClick={onCancel}>
          {t('common.cancel')}
        </Button>
      </div>
    </div>
  );
}
