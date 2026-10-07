import * as React from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input, Textarea } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { ontologyCardApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/api/queries';
import type { Ontology, OntologyCardCheck, OntologyCardContent, OntologyCardVerdict } from '@/lib/api/types';
import { useT, type TranslationKey } from '@/lib/i18n';

/** Ce qu'une ontologie dit du sens des donnees, et ce que la plateforme en
 *  verifie (ADR-047).
 *
 *  Le scan produit un inventaire de chemins — sujets, visites, modalites,
 *  comptes, volumes — et le dit honnetement. Ce qu'il ne peut pas produire,
 *  c'est ce que la donnee *signifie* : sa finalite, ce pour quoi elle ne doit
 *  pas servir, la base legale, les unites, a qui demander. Rien de tout ca ne
 *  se deduit d'un octet, et c'est la plus grande moitie de ce qui manquait.
 *
 *  Sur l'ontologie et non sur le dataset : un dataset est un bucket avec des
 *  identifiants, l'ontologie est la couche qui dit ce qu'il y a dedans. Mettre
 *  la description sur le stockage forcerait de surcroit une seule description
 *  a toutes les ontologies qui lisent le meme bucket.
 *
 *  L'ecran a une regle : une declaration confirmee et une declaration que
 *  personne n'a verifiee ne doivent pas se ressembler. C'est tout l'interet de
 *  la colonne de droite, et c'est pour ca que `unverified` porte un ton
 *  d'avertissement plutot qu'un ton neutre — un lecteur presse lit les couleurs
 *  avant les mots. */

const TONES: Record<OntologyCardVerdict, 'success' | 'danger' | 'warning' | 'neutral' | 'outline'> = {
  agrees: 'success',
  differs: 'danger',
  unverified: 'warning',
  undeclared: 'neutral',
  not_checkable: 'outline',
};

const VERDICT_LABELS: Record<OntologyCardVerdict, TranslationKey> = {
  agrees: 'ontologyCard.verdictAgrees',
  differs: 'ontologyCard.verdictDiffers',
  unverified: 'ontologyCard.verdictUnverified',
  undeclared: 'ontologyCard.verdictUndeclared',
  not_checkable: 'ontologyCard.verdictNotCheckable',
};

const FIELD_LABELS: Record<string, TranslationKey> = {
  subjects: 'ontologyCard.subjects',
  objects: 'ontologyCard.objects',
  modalities: 'ontologyCard.modalities',
  firstVisit: 'ontologyCard.firstVisit',
  lastVisit: 'ontologyCard.lastVisit',
  pseudonymised: 'ontologyCard.pseudonymised',
  purpose: 'ontologyCard.purpose',
  outOfScope: 'ontologyCard.outOfScope',
  limitations: 'ontologyCard.limitations',
  legalBasis: 'ontologyCard.legalBasis',
  consent: 'ontologyCard.consent',
  licence: 'ontologyCard.licence',
};

/** Les champs libres, dans l'ordre ou on les lit : ce que c'est, pour quoi,
 *  d'ou ca vient, comment le lire, a qui demander. */
const PROSE: { key: keyof OntologyCardContent; label: TranslationKey; long?: boolean }[] = [
  { key: 'study', label: 'ontologyCard.study' },
  { key: 'release', label: 'ontologyCard.release' },
  { key: 'population', label: 'ontologyCard.population', long: true },
  { key: 'inclusion', label: 'ontologyCard.inclusion', long: true },
  { key: 'purpose', label: 'ontologyCard.purpose', long: true },
  { key: 'outOfScope', label: 'ontologyCard.outOfScope', long: true },
  { key: 'limitations', label: 'ontologyCard.limitations', long: true },
  { key: 'provenance', label: 'ontologyCard.provenance', long: true },
  { key: 'legalBasis', label: 'ontologyCard.legalBasis' },
  { key: 'consent', label: 'ontologyCard.consent' },
  { key: 'licence', label: 'ontologyCard.licence' },
  { key: 'units', label: 'ontologyCard.units' },
  { key: 'conventions', label: 'ontologyCard.conventions', long: true },
  { key: 'contact', label: 'ontologyCard.contact' },
];

/** Un champ que la table ne connait pas s'affiche tel quel : un libelle
 *  manquant ne doit pas faire disparaitre la ligne de controle qui le porte. */
function labelFor(t: (key: TranslationKey) => string, field: string): string {
  const key = FIELD_LABELS[field];
  return key ? t(key) : field;
}

type Draft = Record<string, string>;

function draftFrom(card: OntologyCardContent | null | undefined): Draft {
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

export function OntologyCardPanel({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const client = useQueryClient();
  const [editing, setEditing] = React.useState(false);
  const [draft, setDraft] = React.useState<Draft>({});

  const card = useQuery({
    queryKey: qk.ontologyCard(ontology.id),
    queryFn: () => ontologyCardApi.get(ontology.id),
    enabled: Boolean(ontology.id),
  });

  const save = useMutation({
    mutationFn: () => ontologyCardApi.set(ontology.id, payloadFrom(draft)),
    onSuccess: () => {
      setEditing(false);
      void client.invalidateQueries({ queryKey: qk.ontologyCard(ontology.id) });
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
          <CardTitle>{t('ontologyCard.title')}</CardTitle>
          <CardDescription>{t('ontologyCard.hint')}</CardDescription>
        </CardHeaderText>
        {!editing ? (
          <Button variant="secondary" onClick={startEditing} disabled={card.isLoading}>
            {card.data?.declared ? t('ontologyCard.edit') : t('ontologyCard.declare')}
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
              <p className="text-sm text-muted-foreground">{t('ontologyCard.undeclaredAll')}</p>
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

function Declared({ content }: { content: OntologyCardContent | null }) {
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
        {t('ontologyCard.versionLine', {
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
  checks: OntologyCardCheck[];
  measuredBy?: string;
  measuredAt?: string;
}) {
  const t = useT();
  if (checks.length === 0) return null;

  // Ce qui demande une action en haut : un ecart, puis une declaration que rien
  // n'a verifiee. Le reste suit.
  const ordre: OntologyCardVerdict[] = ['differs', 'unverified', 'agrees', 'undeclared', 'not_checkable'];
  const tries = [...checks].sort((a, b) => ordre.indexOf(a.verdict) - ordre.indexOf(b.verdict));

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h4 className="text-sm font-medium">{t('ontologyCard.control')}</h4>
        {measuredBy ? (
          <p className="text-xs text-muted-foreground">
            {t('ontologyCard.measuredBy', {
              method: measuredBy,
              when: measuredAt ? new Date(measuredAt).toLocaleString() : '—',
            })}
          </p>
        ) : (
          <p className="text-xs text-muted-foreground">{t('ontologyCard.neverMeasured')}</p>
        )}
      </div>

      <ul className="divide-y divide-border">
        {tries.map((check) => (
          <li key={check.field} className="flex flex-wrap items-baseline gap-x-3 gap-y-1 py-2">
            <span className="min-w-32 text-sm">{labelFor(t, check.field)}</span>
            <Badge tone={TONES[check.verdict]}>{t(VERDICT_LABELS[check.verdict])}</Badge>
            {check.declared ? (
              <span className="text-xs text-muted-foreground">
                {t('ontologyCard.declaredValue')} <span className="font-mono">{check.declared}</span>
              </span>
            ) : null}
            {check.measured ? (
              <span className="text-xs text-muted-foreground">
                {t('ontologyCard.measuredValue')} <span className="font-mono">{check.measured}</span>
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
        <h4 className="text-sm font-medium">{t('ontologyCard.claims')}</h4>
        <p className="text-xs text-muted-foreground">{t('ontologyCard.claimsHint')}</p>
        <div className="grid gap-3 sm:grid-cols-3">
          <Field label={t('ontologyCard.subjects')}>
            <Input
              inputMode="numeric"
              value={draft.subjects ?? ''}
              onChange={(event) => onChange('subjects', event.target.value)}
            />
          </Field>
          <Field label={t('ontologyCard.objects')}>
            <Input
              inputMode="numeric"
              value={draft.objects ?? ''}
              onChange={(event) => onChange('objects', event.target.value)}
            />
          </Field>
          <Field
            label={t('ontologyCard.pseudonymised')}
            description={t('ontologyCard.pseudonymisedHint')}
          >
            <Select
              value={draft.pseudonymised ?? ''}
              onValueChange={(value) => onChange('pseudonymised', value)}
              placeholder={t('ontologyCard.notStated')}
              options={[
                { value: 'true', label: t('common.yes') },
                { value: 'false', label: t('common.no') },
              ]}
            />
          </Field>
          <Field label={t('ontologyCard.modalities')} className="sm:col-span-3">
            <Input
              value={draft.modalities ?? ''}
              onChange={(event) => onChange('modalities', event.target.value)}
              placeholder="DICOM, OCT"
            />
          </Field>
          <Field label={t('ontologyCard.firstVisit')}>
            <Input
              value={draft.firstVisit ?? ''}
              onChange={(event) => onChange('firstVisit', event.target.value)}
              placeholder="AAAAMMJJ"
            />
          </Field>
          <Field label={t('ontologyCard.lastVisit')}>
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
