import * as React from 'react';
import { Link } from 'react-router';
import { useMutation, useQuery } from '@tanstack/react-query';
import {
  AlertTriangle,
  FolderOpen,
  MessageCircle,
  Network,
  ChevronRight,
  Radar,
  Search,
  Trash2,
} from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { OwnerTransfer, ResourceOwner } from '@/components/common/owner';
import { ProjectMountsSheet } from '@/components/common/project-mounts';
import { useConfirm } from '@/components/common/confirm-dialog';
import { SectionHeader } from '@/components/common/page-header';
import { assistantAvailable, requestAssistant } from '@/lib/assistant-bridge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardHeaderText,
  CardTitle,
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrapper,
} from '@/components/ui/table';
import { Sheet, SheetBody, SheetContent } from '@/components/ui/sheet';
import { useToast } from '@/components/ui/toast';
import {
  useOntologies,
  useOntologyFreshness,
  useOntologyCompleteness,
  useOntologyScans,
  useOntologyExtracts,
  useDatasets,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { deletionCostApi, ontologiesApi, pathLayoutApi } from '@/lib/api/endpoints';
import { OntologyCardPanel } from './ontology-card';
import { useI18n, useT, type TranslationKey } from '@/lib/i18n';
import { capitaliser, motsDuMetier, pluriel } from './ontology-words';
import { formatNumber, formatRelative } from '@/lib/format';
import type {
  Extract,
  OntologyQueryItem,
  Ontology,
  OntologyScan,
  DatasetPathLayout,
  DatasetPathLayoutTrial,
} from '@/lib/api/types';

/* Ce que le manifeste porte et que cet ecran lit.
 *
 *  Le type complet vit cote serveur ; declarer ici le strict necessaire evite
 *  de le dupliquer et dit en meme temps ce dont l'assistant a besoin. */
/** Comment appeler les trois niveaux, dans le metier qui possede la donnee.
 *
 *  Les positions etaient configurables et les mots ne l'etaient pas, donc une
 *  plateforme vendue aussi a des banques et a des assureurs affichait
 *  « sujets », « visites » et « modalites » sur une table d'operations. Ce
 *  n'est pas un probleme de purete : c'est une demo qui perd la salle, parce
 *  que le produit appartient visiblement au metier de quelqu'un d'autre.
 *
 *  Les defauts sont generiques plutot que cliniques : un niveau sans nom est
 *  une clef de regroupement, un axe temporel et une sorte, quel que soit le
 *  metier. */
/** Un pluriel suffisant pour un libelle : les mots du metier sont des noms
 *  communs courts, et un « s » tient pour « entités », « clients »,
 *  « patients ». Un mot deja au pluriel n'en gagne pas un second. */
/* La regle de lecture, en francais, plutot que la prose du serveur.
 *
 *  Le bandeau affichait « the platform's compiled rule: the first segment that
 *  looks like a subject identifier, then the next two as visit and modality » :
 *  un commentaire de code servi a l'utilisateur. La regle est trois positions
 *  et trois mots ; elle se reconstitue ici, dans la langue de l'ecran. */
function regleLisible(
  ontology: Ontology | null | undefined,
  t: (key: TranslationKey, values?: Record<string, string | number>) => string,
): string {
  const regle = (ontology?.manifest as ManifestLu | undefined)?.readingRule;
  if (!regle || regle.source !== 'declared') return t('ontologies.patternRuleDefaultPlain');
  const mots = motsDuMetier(ontology);
  const niveaux = [
    regle.subjectLevel !== undefined && regle.subjectLevel >= 0
      ? t('ontologies.patternLevelIs', { level: String(regle.subjectLevel), name: mots.sujet })
      : null,
    regle.visitLevel !== undefined && regle.visitLevel >= 0
      ? t('ontologies.patternLevelIs', { level: String(regle.visitLevel), name: mots.visite })
      : null,
    regle.modalityLevel !== undefined && regle.modalityLevel >= 0
      ? t('ontologies.patternLevelIs', { level: String(regle.modalityLevel), name: mots.modalite })
      : null,
  ].filter(Boolean);
  return niveaux.join(', ');
}

type ManifestLu = {
  subjects?: { id?: string }[];
  summary?: {
    objects?: number;
    subjects?: number;
    unrecognisedObjects?: number;
    recognisedLayouts?: string[];
    layoutSamples?: string[];
    directoryKeys?: number;
  };
  /** La regle qui a produit CETTE photographie, et non le nom d'un profil qui
   *  a pu changer depuis (ADR-040). */
  readingRule?: {
    source?: string;
    description?: string;
    subjectLevel?: number;
    visitLevel?: number;
    modalityLevel?: number;
    /** Les mots du metier qui possede la donnee, tels qu'ils etaient quand
     *  cette photographie a ete prise. Enregistres avec la regle et pour la
     *  meme raison (ADR-040) : un nom change ensuite ferait decrire une vieille
     *  photographie dans un vocabulaire avec lequel elle n'a jamais ete lue. */
    subjectName?: string;
    visitName?: string;
    modalityName?: string;
  };
};

/**
 * Ontology catalogue (ADR-025).
 *
 * The natural-language query surface keeps the guardrail the ADR asks for:
 * the generated result is presented as data to inspect, never as an
 * authoritative answer, and the ontology's own manifest stays visible.
 */
function OntologyQuery({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const toast = useToast();
  const [question, setQuestion] = React.useState('');
  const [result, setResult] = React.useState<{
    items?: OntologyQueryItem[];
    count?: number;
    limited?: boolean;
  } | null>(null);

  const run = useMutation({
    mutationFn: () => ontologiesApi.query(ontology.id, question.trim()),
    onSuccess: (response) => {
      if (response.error) {
        toast.error(response.error, t('ontologies.query'));
        setResult(null);
        return;
      }
      setResult(response);
    },
    onError: (error) => toast.error(error, t('ontologies.query')),
  });

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>
            {t('ontologies.query')} — {ontology.name}
          </CardTitle>
          <CardDescription>
            {t('ontologies.queryHint', {
              entity: motsDuMetier(ontology).sujet,
              period: motsDuMetier(ontology).visite,
              category: motsDuMetier(ontology).modalite,
            })}
          </CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (question.trim()) run.mutate();
          }}
          className="flex items-end gap-2"
        >
          <Field label={t('ontologies.queryLabel')} className="flex-1">
            <Input
              value={question}
              onChange={(event) => setQuestion(event.target.value)}
              placeholder={t('ontologies.queryPlaceholder')}
            />
          </Field>
          <Button type="submit" variant="primary" loading={run.isPending} disabled={!question.trim()}>
            <Search aria-hidden />
            {t('ontologies.query')}
          </Button>
        </form>

        {result ? (
          result.items?.length ? (
            <>
              <p className="text-xs text-muted-foreground">
                {t('ontologies.matches', {
                  shown: String(result.items.length),
                  total: String(result.count ?? result.items.length),
                })}
              </p>
              <TableWrapper className="rounded-md border border-border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('ontologies.object')}</TableHead>
                      <TableHead>{t('common.type')}</TableHead>
                      <TableHead>{t('ontologies.parent')}</TableHead>
                      <TableHead className="text-right">{t('ontologies.objects')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {result.items.map((item) => (
                      <TableRow key={`${item.parent}/${item.object}/${item.type}`}>
                        <TableCell className="font-mono text-xs">{item.object}</TableCell>
                        <TableCell className="text-xs">{item.type}</TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {item.parent || '—'}
                        </TableCell>
                        <TableCell className="text-right text-xs tabular-nums">{item.count}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableWrapper>
            </>
          ) : (
            <EmptyState title={t('ontologies.noMatch')} />
          )
        ) : null}
      </CardContent>
    </Card>
  );
}

/**
 * Whether the ontology still describes its source.
 *
 * An ontology is a photograph, and the screen presented it as a fact: the June
 * scan of the study read "18,738 objects, 20 subjects" in exactly the same
 * typeface as this morning's 24,179 and 31, and nothing said the study had
 * recruited eleven subjects in between. Everything built on an ontology - a
 * extract above all - silently inherits that gap, so the count is checked
 * against the source and the difference is stated in objects.
 */
function OntologyFreshnessNote({ ontologyId }: { ontologyId: string }) {
  const t = useT();
  const { locale } = useI18n();
  const freshness = useOntologyFreshness(ontologyId);
  const data = freshness.data;
  if (!data) return null;

  const measured =
    data.ageDays > 0
      ? t('ontologies.freshnessMeasured', {
          days: String(data.ageDays),
          objects: formatNumber(data.manifestObjects, locale),
        })
      : t('ontologies.freshnessMeasuredToday', {
          objects: formatNumber(data.manifestObjects, locale),
        });

  // A source that could not be reached leaves the count unknown rather than
  // zero: "24,179 fewer objects" would be a frightening lie.
  const drift =
    data.sourceObjects === 0 && data.manifestObjects > 0
      ? t('ontologies.freshnessUnknown')
      : data.drift > 0
        ? t('ontologies.freshnessGrown', { drift: formatNumber(data.drift, locale) })
        : data.drift < 0
          ? t('ontologies.freshnessShrunk', { drift: formatNumber(-data.drift, locale) })
          : t('ontologies.freshnessAligned');

  return (
    <div
      className={
        data.stale
          ? 'flex items-start gap-2 rounded-md border border-warning/40 bg-warning-subtle px-3 py-2'
          : 'flex items-start gap-2 rounded-md border border-border px-3 py-2'
      }
    >
      {data.stale ? (
        <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0 text-warning-foreground" />
      ) : null}
      <p className={data.stale ? 'text-xs leading-relaxed text-warning-foreground' : 'text-xs text-muted-foreground'}>
        {data.stale ? (
          <span className="font-medium">{t('ontologies.freshnessStale')} — </span>
        ) : null}
        {measured} {drift}
      </p>
    </div>
  );
}

/**
 * Who the study covers, and who an extract would leave out.
 *
 * "31 subjects" was the only number on the screen, and it averaged together
 * the subjects who carry a corneal wavefront and those who do not. Anyone
 * assembling an extract by modality needs the second list by name, before they
 * publish an n.
 */
/**
 * Comment la plateforme lit cette source.
 *
 * Le pattern d'entree : quelle position d'un chemin porte le sujet, laquelle
 * la date de visite, laquelle la modalite. C'est ce que le profil d'inference
 * decide, et c'est ce que l'assistant propose quand on lui montre les formes.
 *
 * L'ecran n'en affichait que le nom - "health-file-path-v1" - qui ne dit rien
 * a personne. Les formes reconnues, elles, le disent exactement : elles sont
 * le pattern, mesure sur cette source et non promis en general. Et les formes
 * qui n'ont rien produit disent ce qui est passe a cote, ce qui est la seule
 * facon de savoir si la lecture est juste.
 */
function demanderUneLecture(ontology: Ontology, regle: DatasetPathLayout | undefined, invite: string) {
    const resume = (ontology.manifest as ManifestLu | undefined)?.summary;
    requestAssistant({
      surface: 'ontology',
      context: {
        ontologyId: ontology.id,
        ontologyName: ontology.name,
        sourceName: ontology.sourceName,
        inferenceProfile: ontology.inferenceProfile,
        objects: resume?.objects,
        subjects: resume?.subjects,
        unrecognisedObjects: resume?.unrecognisedObjects,
        // Celles qui ont produit un sujet : elles disent a quoi ressemble le
        // jeu de donnees.
        recognisedLayouts: resume?.recognisedLayouts ?? [],
        // Et celles qui n'ont rien produit : elles disent ce qui est manque.
        unrecognisedLayouts: resume?.layoutSamples ?? [],
        // La regle deja appliquee, s'il y en a une : proposer une lecture sans
        // savoir laquelle est en place, c'est proposer a l'aveugle.
        currentRule: regle?.declared ? regle.description : '',
      },
      prompt: invite,
    });

}

/* Comment le dataset choisi sera lu.
 *
 *  Avant de lancer un scan, la seule question utile est celle-la. Le dataset
 *  porte sa propre regle, ou il n'en porte pas et c'est la regle par defaut de
 *  la plateforme qui s'applique - laquelle attend <sujet>/<visite>/<modalite>
 *  et compte tout le reste comme non reconnu. Dans les deux cas, dire laquelle
 *  evite le scan qu'on relance trois fois en se demandant pourquoi. */
function ScanRule({ datasetId }: { datasetId: string }) {
  const t = useT();
  const regle = useQuery({
    queryKey: qk.datasetPathLayout(datasetId),
    queryFn: () => pathLayoutApi.get(datasetId),
    enabled: Boolean(datasetId),
  });

  if (regle.isLoading) return null;
  const declaree = regle.data?.declared;

  return (
    <p className="text-xs text-muted-foreground">
      {declaree ? (
        <>
          <span className="font-medium">{t('ontologies.scanRuleDeclared')}</span>{' '}
          <span className="font-mono">{regle.data?.description}</span>{' '}
          {t('ontologies.scanRuleWhereToChange')}
        </>
      ) : (
        <>
          <span className="font-medium">{t('ontologies.scanRuleDefault')}</span>{' '}
          {t('ontologies.scanRuleDefaultHint')}
        </>
      )}
    </p>
  );
}

/* L'editeur de la regle de lecture.
 *
 *  Trois niveaux, saisis a la main, essayes sur de vrais chemins avant d'etre
 *  enregistres. C'est ce qui rend la regle utilisable sans assistant : trois
 *  nombres sont abstraits, "niveau 1 lit SELENA-01-001 sur ce chemin" se
 *  verifie en une seconde par quelqu'un qui connait l'etude.
 *
 *  L'essai porte de vrais chemins et ne part donc jamais vers un modele : sur
 *  ces jeux de donnees une cle est un identifiant de patient, et c'est
 *  exactement pour ca qu'on montre des formes a l'assistant (ADR-040). */
function PatternEditor({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const datasetId = ontology.sourceId;
  const current = useQuery({
    queryKey: qk.datasetPathLayout(datasetId),
    queryFn: () => pathLayoutApi.get(datasetId),
    enabled: Boolean(datasetId),
  });

  const [subject, setSubject] = React.useState('');
  const [visit, setVisit] = React.useState('');
  const [modality, setModality] = React.useState('');
  const [nomSujet, setNomSujet] = React.useState('');
  const [nomVisite, setNomVisite] = React.useState('');
  const [nomModalite, setNomModalite] = React.useState('');
  const [trial, setTrial] = React.useState<DatasetPathLayoutTrial | null>(null);

  React.useEffect(() => {
    const data = current.data;
    if (!data?.declared) return;
    setSubject(String(data.subjectLevel ?? ''));
    setVisit(data.visitLevel != null && data.visitLevel >= 0 ? String(data.visitLevel) : '');
    setModality(
      data.modalityLevel != null && data.modalityLevel >= 0 ? String(data.modalityLevel) : '',
    );
    setNomSujet(data.subjectName ?? '');
    setNomVisite(data.visitName ?? '');
    setNomModalite(data.modalityName ?? '');
  }, [current.data]);

  const niveaux = () => ({
    subjectLevel: subject.trim() === '' ? null : Number(subject),
    visitLevel: visit.trim() === '' ? null : Number(visit),
    modalityLevel: modality.trim() === '' ? null : Number(modality),
    /* Les mots du metier, a cote des positions.
     *
     *  Trois champs qu'on remplit une fois, ou que l'assistant propose avec la
     *  regle. Laisses vides, la plateforme reste generique - entite, periode,
     *  categorie - plutot que de parler clinique a une banque. */
    subjectName: nomSujet.trim(),
    visitName: nomVisite.trim(),
    modalityName: nomModalite.trim(),
  });

  const essai = useMutation({
    mutationFn: () => pathLayoutApi.try(datasetId, niveaux()),
    onSuccess: (data) => setTrial(data),
    onError: (error) => toast.error(error, t('ontologies.patternTry')),
  });

  const enregistrer = useMutation({
    mutationFn: () => pathLayoutApi.set(datasetId, niveaux()),
    onSuccess: () => {
      invalidate(qk.datasetPathLayout(datasetId));
      toast.success(t('ontologies.patternSaved'));
    },
    onError: (error) => toast.error(error, t('ontologies.patternSave')),
  });

  const oublier = useMutation({
    mutationFn: () => pathLayoutApi.set(datasetId, {}),
    onSuccess: () => {
      invalidate(qk.datasetPathLayout(datasetId));
      setSubject('');
      setVisit('');
      setModality('');
      setTrial(null);
      toast.success(t('ontologies.patternCleared'));
    },
    onError: (error) => toast.error(error, t('ontologies.patternClear')),
  });

  if (!datasetId) return null;

  return (
    <div className="space-y-3 rounded-md border border-border p-3">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs font-medium">{t('ontologies.patternRule')}</p>
        <span className="text-xs text-muted-foreground">
          {current.data?.declared ? current.data.description : t('ontologies.patternCompiled')}
        </span>
      </div>
      <p className="text-xs text-muted-foreground">{t('ontologies.patternRuleHint')}</p>

      <div className="grid gap-2 sm:grid-cols-3">
        <Field label={t('ontologies.patternSubjectLevel')}>
          <Input value={subject} onChange={(event) => setSubject(event.target.value)} placeholder="1" />
        </Field>
        <Field label={t('ontologies.patternVisitLevel')}>
          <Input value={visit} onChange={(event) => setVisit(event.target.value)} placeholder="2" />
        </Field>
        <Field label={t('ontologies.patternModalityLevel')}>
          <Input value={modality} onChange={(event) => setModality(event.target.value)} placeholder="3" />
        </Field>
      </div>

      {/* Les mots du metier qui possede la donnee. Vides, la plateforme reste
          generique plutot que de parler clinique a une banque. */}
      <p className="text-xs text-muted-foreground">{t('ontologies.patternNamesHint')}</p>
      <div className="grid gap-2 sm:grid-cols-3">
        <Field label={t('ontologies.patternSubjectName')}>
          <Input value={nomSujet} onChange={(event) => setNomSujet(event.target.value)} placeholder="patient, client…" />
        </Field>
        <Field label={t('ontologies.patternVisitName')}>
          <Input value={nomVisite} onChange={(event) => setNomVisite(event.target.value)} placeholder="visite, mois…" />
        </Field>
        <Field label={t('ontologies.patternModalityName')}>
          <Input value={nomModalite} onChange={(event) => setNomModalite(event.target.value)} placeholder="modalité, opération…" />
        </Field>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button
          variant="secondary"
          loading={essai.isPending}
          disabled={subject.trim() === ''}
          onClick={() => essai.mutate()}
        >
          {t('ontologies.patternTry')}
        </Button>
        <Button
          variant="primary"
          loading={enregistrer.isPending}
          disabled={subject.trim() === ''}
          onClick={() => enregistrer.mutate()}
        >
          {t('ontologies.patternSave')}
        </Button>
        {current.data?.declared ? (
          <Button variant="ghost" loading={oublier.isPending} onClick={() => oublier.mutate()}>
            {t('ontologies.patternClear')}
          </Button>
        ) : null}
        {/* L'assistance, a cote des champs qu'elle remplit.
          *
          *  Il proposait deja une lecture, et il n'existait aucun endroit pour
          *  ecrire sa reponse : la conversation finissait dans le panneau. Le
          *  bouton est ici parce que c'est ici qu'on transcrit les trois
          *  nombres - et il reste facultatif, la regle se saisit a la main. */}
        {assistantAvailable() ? (
          <Button variant="ghost" onClick={() => {
              demanderUneLecture(ontology, current.data, t('ontologies.describePrompt', { name: ontology.name }));
              toast.success(t('ontologies.describeSent'), t('ontologies.describe'));
            }}>
            <MessageCircle aria-hidden />
            {t('ontologies.patternAsk')}
          </Button>
        ) : null}
      </div>

      {trial ? (
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">
            {t('ontologies.patternTrialResult', {
              recognised: String(trial.recognised),
              sampled: String(trial.sampled),
            })}
          </p>
          <TableWrapper className="rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('ontologies.object')}</TableHead>
                  <TableHead>{t('ontologies.patternSubject')}</TableHead>
                  <TableHead>{t('ontologies.patternVisit')}</TableHead>
                  <TableHead>{t('ontologies.patternModality')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {trial.items.map((item) => (
                  <TableRow key={item.path}>
                    <TableCell className="max-w-[20rem] truncate font-mono text-xs">
                      {item.path}
                    </TableCell>
                    <TableCell className="font-mono text-xs">{item.subject || '—'}</TableCell>
                    <TableCell className="font-mono text-xs">{item.visit || '—'}</TableCell>
                    <TableCell className="font-mono text-xs">{item.modality || '—'}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableWrapper>
        </div>
      ) : null}
    </div>
  );
}

function OntologyPattern({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const { locale } = useI18n();
  const resume = (ontology.manifest as ManifestLu | undefined)?.summary;
  const reconnues = resume?.recognisedLayouts ?? [];
  const refusees = resume?.layoutSamples ?? [];
  const dossiers = resume?.directoryKeys ?? 0;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.pattern')}</CardTitle>
          <CardDescription>{t('ontologies.patternHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          {/* Nomme, pas pose nu. "health-file-path-v1" seul sur une ligne
            *  ne dit rien a personne ; ce qu'il faut savoir est que cette
            *  photographie-ci a ete prise avec cette regle-la. */}
          <span>{t('ontologies.patternProducedWith')}</span>
          <Badge tone="outline">{regleLisible(ontology, t)}</Badge>
          {dossiers > 0 ? (
            <span>{t('ontologies.patternDirectories', { count: formatNumber(dossiers, locale) })}</span>
          ) : null}
        </div>

        {reconnues.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('ontologies.patternNone')}</p>
        ) : (
          <div>
            <p className="mb-1 text-xs font-medium">
              {t('ontologies.patternRecognisedFor', { entities: pluriel(motsDuMetier(ontology).sujet) })}
            </p>
            <FormesListe formes={reconnues} />
          </div>
        )}

        <PatternEditor ontology={ontology} />

        {refusees.length > 0 ? (
          <div>
            <p className="mb-1 text-xs font-medium">{t('ontologies.patternUnrecognised')}</p>
            <FormesListe formes={refusees} />
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

/* Une liste de formes qui avoue sa longueur.
 *
 *  Le serveur n'en renvoyait que huit, sans le dire : sur SELENA elles
 *  totalisaient 3 979 objets sur 3 994, et les quinze autres n'apparaissaient
 *  nulle part. Une liste qui a l'air exhaustive et ne l'est pas est pire
 *  qu'une liste courte, parce que personne ne pense a demander ce qui manque.
 *  On en montre huit - au-dela, un ecran ne se lit plus - et on dit combien
 *  restent, avec de quoi les voir. */
function FormesListe({ formes }: { formes: string[] }) {
  const t = useT();
  const { locale } = useI18n();
  const [tout, setTout] = React.useState(false);
  const visibles = tout ? formes : formes.slice(0, 8);
  const reste = formes.length - visibles.length;

  return (
    <>
      <ul className="space-y-0.5">
        {visibles.map((forme) => (
          <li key={forme} className="font-mono text-xs text-muted-foreground">
            {forme}
          </li>
        ))}
      </ul>
      {reste > 0 || tout ? (
        <button
          type="button"
          onClick={() => setTout((current) => !current)}
          className="mt-1 text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
        >
          {tout
            ? t('ontologies.patternShapesFewer')
            : t('ontologies.patternShapesMore', { count: formatNumber(reste, locale) })}
        </button>
      ) : null}
    </>
  );
}

function OntologyCoverage({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const { locale } = useI18n();
  const mots = motsDuMetier(ontology);
  const ontologyId = ontology.id;
  const coverage = useOntologyCompleteness(ontologyId);
  const data = coverage.data;
  if (!data || data.modalities.length === 0) return null;

  // Un controle qui passe n'est pas un rapport.
  //
  //  Le bloc existe pour une seule raison : prevenir qu'un extrait demande
  //  par categorie exclut en silence les entites qui ne la portent pas. Sur
  //  SELENA tout etait a 2/2 et « Aucun » partout - cinq lignes de tableau
  //  pour dire qu'il n'y a rien a dire, juste au-dessus d'une phrase qui le
  //  disait deja. Il ne s'affiche donc que lorsqu'il a quelque chose a
  //  signaler, et la phrase rassurante suffit le reste du temps.
  const trous = data.modalities.filter((modality) => modality.missingSubjects.length > 0);
  if (trous.length === 0) return null;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.completeness')}</CardTitle>
          <CardDescription>
            {t('ontologies.completenessHint', {
              entities: pluriel(mots.sujet),
              category: mots.modalite,
            })}
          </CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          {t('ontologies.completenessSubjects', {
            entities: pluriel(mots.sujet),
            category: mots.modalite,
            categories: pluriel(mots.modalite),
            complete: formatNumber(data.completeSubjects, locale),
            total: formatNumber(data.subjects, locale),
          })}
        </p>
        <TableWrapper className="rounded-md border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('ontologies.completenessModality')}</TableHead>
                <TableHead className="text-right">{capitaliser(pluriel(mots.sujet))}</TableHead>
                <TableHead className="text-right">{t('ontologies.objects')}</TableHead>
                <TableHead>{t('ontologies.completenessWithout', { entities: pluriel(mots.sujet), category: mots.modalite })}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {trous.map((modality) => (
                <TableRow key={modality.name}>
                  <TableCell className="font-mono text-xs">{modality.name}</TableCell>
                  <TableCell className="text-right text-xs tabular-nums">
                    {formatNumber(modality.subjects, locale)} / {formatNumber(data.subjects, locale)}
                  </TableCell>
                  <TableCell className="text-right text-xs tabular-nums">
                    {formatNumber(modality.objects, locale)}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    <span className="font-mono">{modality.missingSubjects.join(', ')}</span>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableWrapper>
      </CardContent>
    </Card>
  );
}

/**
 * Extracts: the subset a study is actually run on.
 *
 * Declaring one resolves the selection to an explicit list of files and keeps
 * it. An extract that re-ran its filter would return a different study every
 * month, and last month's n would stop being reproducible.
 *
 * Nothing is duplicated: the frozen paths point into the dataset where the data
 * already lives, and a workspace mounts the extract as a tree of links over it.
 */
/** Ceder un extrait, dans la meme forme et les memes mots qu'un dataset.
 *
 *  Un extrait est une selection gelee : elle survit aux projets qui s'en sont
 *  servis, et souvent a la personne qui l'a declaree. La ceder est ce qui
 *  permet a une equipe de garder ce qu'elle a produit quand l'un d'eux s'en
 *  va. */
/** Corriger le libelle d'un extrait.
 *
 *  Le nom est tape a la declaration - "anterion-3-sujets" - donc il se tape
 *  mal, et il etait immuable : la seule issue etait de redeclarer la
 *  selection, ce qui fige une autre liste sur un bucket qui grandit. Un
 *  renommage devenait une autre etude.
 *
 *  Le libelle bouge, rien d'autre : la liste de fichiers, l'auteur et les
 *  dates sont ce qui fait l'extrait, et un n deja publie sous ce nom decrit
 *  toujours les memes fichiers. */
export function ExtractRenameSheet({
  extract,
  open,
  onOpenChange,
}: {
  extract: Extract | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const [name, setName] = React.useState('');
  const [description, setDescription] = React.useState('');

  // Suivre la ligne ouverte, sinon la feuille propose de renommer l'extrait
  // precedent avec le nom de celui qu'on vient d'ouvrir.
  React.useEffect(() => {
    setName(extract?.name ?? '');
    setDescription(extract?.description ?? '');
  }, [extract?.id, extract?.name, extract?.description]);

  const rename = useMutation({
    mutationFn: () =>
      ontologiesApi.updateExtract(extract?.id ?? '', {
        name: name.trim(),
        description: description.trim(),
      }),
    onSuccess: () => {
      invalidate(qk.extracts);
      invalidate(qk.ontologies);
      toast.success(t('ontologies.extractRename'));
      onOpenChange(false);
    },
    onError: (error) => toast.error(error, t('ontologies.extractRename')),
  });

  const unchanged =
    name.trim() === (extract?.name ?? '') && description.trim() === (extract?.description ?? '');

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent title={extract?.name ?? t('ontologies.extractRename')}>
        <SheetBody>
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              if (name.trim() && !unchanged) rename.mutate();
            }}
          >
            <Field label={t('common.name')} description={t('ontologies.extractRenameHint')}>
              <Input value={name} onChange={(event) => setName(event.target.value)} />
            </Field>
            <Field label={t('common.description')}>
              <Input
                value={description}
                onChange={(event) => setDescription(event.target.value)}
              />
            </Field>
            <Button
              type="submit"
              variant="primary"
              loading={rename.isPending}
              disabled={!name.trim() || unchanged}
            >
              {t('common.save')}
            </Button>
          </form>
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}

export function ExtractOwnershipSheet({
  extract,
  open,
  onOpenChange,
}: {
  extract: Extract | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();

  const transfer = useMutation({
    mutationFn: (input: { ownerType: string; ownerId: string }) =>
      ontologiesApi.setExtractOwner(extract?.id ?? '', input),
    onSuccess: () => {
      invalidate(qk.ontologies);
      invalidate(qk.extracts);
      toast.success(t('projects.transferOwnership'));
      onOpenChange(false);
    },
    onError: (error) => toast.error(error, t('projects.transferOwnership')),
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent title={extract?.name ?? t('projects.transferOwnership')}>
        <SheetBody>
          {extract ? (
            <OwnerTransfer
              owner={extract}
              pending={transfer.isPending}
              onTransfer={(input) => transfer.mutate(input)}
              label={t('projects.transferOwnership')}
            />
          ) : null}
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}


/**
 * Launching a scan.
 *
 * The endpoint existed, the API client had a function for it, and no screen
 * called either - so an installation with no ontology had no way to make one
 * except by hand against the API, and the empty state said "no ontology" as if
 * that were a fact about the data rather than a missing button. The client also
 * sent no body, and the scan needs to be told which dataset to read.
 *
 * The scan reads object *paths*, never their content, and creates a new
 * ontology rather than replacing one: a scan is a photograph, and photographs
 * do not overwrite each other.
 */
function OntologyScan() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const [datasetId, setDatasetId] = React.useState('');
  /* Les datasets du catalogue, pas ceux d'un projet.
   *
   *  Le formulaire demandait un projet d'abord, puis le dataset dans ce
   *  projet - et la question laissait perplexe, a juste titre : une ontologie
   *  decrit la mise en page d'un jeu de donnees, qui ne depend d'aucun projet.
   *  Le projet ne sert qu'a y rattacher le resultat, ce qui est une seconde
   *  question et pas un prealable. Le serveur accepte d'ailleurs un scan sans
   *  projet depuis le 2026-09-29. */
  const datasets = useDatasets();

  const scan = useMutation({
    mutationFn: () => ontologiesApi.scanOntology({ datasetId }),
    onSuccess: (response) => {
      const summary = response.manifest?.summary;
      // Cree ou rafraichie : la personne doit savoir laquelle des deux, sinon
      // elle compte les lignes. Un second scan de la meme source remplace
      // desormais la photographie et garde l'objet.
      toast.success(
        t(response.refreshed ? 'ontologies.scanRefreshed' : 'ontologies.scanDone', {
          objects: formatNumber(summary?.objects ?? 0, locale),
          subjects: formatNumber(summary?.subjects ?? 0, locale),
          // Les mots de la photographie qui vient d'etre prise, pas ceux du
          // dataset tel qu'il est etiquete aujourd'hui.
          entities: pluriel(motsDuMetier(response.item).sujet),
        }),
      );
      // Et pourquoi ca differe, quand ca differe.
      //
      // Un compte de sujets qui passe de 2 a 31 est soit l'etude qui recrute,
      // soit une regle que quelqu'un vient d'editer - deux nouvelles
      // differentes, qui appellent des reactions opposees. Donner les chiffres
      // sans la cause laisse deviner.
      const diff = response.changed;
      if (diff && (diff.previousObjects !== diff.currentObjects || diff.previousSubjects !== diff.currentSubjects)) {
        toast.success(
          t(diff.readingChanged ? 'ontologies.scanChangedReading' : 'ontologies.scanChangedData', {
            entities: pluriel(motsDuMetier(response.item).sujet),
            objectsBefore: formatNumber(diff.previousObjects, locale),
            objectsAfter: formatNumber(diff.currentObjects, locale),
            subjectsBefore: formatNumber(diff.previousSubjects, locale),
            subjectsAfter: formatNumber(diff.currentSubjects, locale),
          }),
          t('ontologies.scanChangedTitle'),
        );
      }
      invalidate(qk.ontologies);
    },
    onError: (error) => toast.error(error, t('ontologies.scanTitle')),
  });

  const datasetOptions = (datasets.data ?? []).map((item) => ({
    value: item.id,
    label: item.name,
    hint: item.bucket,
  }));

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.scanTitle')}</CardTitle>
          <CardDescription>{t('ontologies.scanHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-3">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (datasetId) scan.mutate();
          }}
          className="grid gap-2 sm:grid-cols-[1fr_auto] sm:items-end"
        >
          <Field label={t('ontologies.scanDataset')}>
            <Select
              value={datasetId}
              onValueChange={setDatasetId}
              placeholder={t('ontologies.scanDataset')}
              disabled={datasetOptions.length === 0}
              options={datasetOptions}
            />
          </Field>
          <Button
            type="submit"
            variant="primary"
            loading={scan.isPending}
            disabled={!datasetId}
          >
            <Radar aria-hidden />
            {t('ontologies.scan')}
          </Button>
        </form>
        {datasetOptions.length === 0 && !datasets.isLoading ? (
          <EmptyState title={t('ontologies.scanNoDataset')} />
        ) : null}
        {/* La regle qui sera appliquee a CE dataset, pas un paragraphe sur un
            identifiant interne.
          *
          *  L'ecran nommait "health-file-path-v1" et decrivait ce qu'il
          *  reconnait - un nom qui ne dit rien a personne, et une description
          *  devenue fausse le jour ou un dataset a pu porter sa propre regle.
          *  Ce qui compte avant de lancer un scan est : comment celui-ci
          *  va-t-il etre lu, et ou le corriger. */}
        {datasetId ? <ScanRule datasetId={datasetId} /> : null}
      </CardContent>
    </Card>
  );
}

/** Handing an ontology over, in the same form and the same words as a dataset
 *  or a project: transferring is one gesture, not three to learn. */
/* Corriger ce que le scan a lu.
 *
 *  La route existait depuis le premier jour et aucun ecran ne l'appelait : le
 *  nom vient de la source, et quand il en sort faux - "SELENA-01" pour une
 *  etude appelee SELENA-001 - la seule issue etait de supprimer l'ontologie et
 *  de rescanner, ce qui en recree une autre et laisse la premiere derriere.
 *
 *  Le nom est un libelle, pas une cle : les objets, les extraits et les droits
 *  pendent de l'identifiant, donc le corriger ne casse rien. */
function OntologyNaming({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const [name, setName] = React.useState(ontology.name);
  const [description, setDescription] = React.useState(ontology.description ?? '');

  // Suivre la ligne selectionnee, sinon le formulaire propose de renommer
  // l'ontologie precedente avec le nom de celle qu'on vient d'ouvrir.
  React.useEffect(() => {
    setName(ontology.name);
    setDescription(ontology.description ?? '');
  }, [ontology.id, ontology.name, ontology.description]);

  const update = useMutation({
    mutationFn: () => ontologiesApi.update(ontology.id, { name: name.trim(), description: description.trim() }),
    onSuccess: () => {
      invalidate(qk.ontologies);
      toast.success(t('ontologies.rename'));
    },
    onError: (error) => toast.error(error, t('ontologies.rename')),
  });

  const unchanged = name.trim() === ontology.name && description.trim() === (ontology.description ?? '');

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.rename')}</CardTitle>
          <CardDescription>{t('ontologies.renameHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (name.trim() && !unchanged) update.mutate();
          }}
          className="grid gap-2 sm:grid-cols-[1fr_2fr_auto] sm:items-end"
        >
          <Field label={t('common.name')}>
            <Input value={name} onChange={(event) => setName(event.target.value)} />
          </Field>
          <Field label={t('common.description')}>
            <Input value={description} onChange={(event) => setDescription(event.target.value)} />
          </Field>
          <Button
            type="submit"
            variant="primary"
            loading={update.isPending}
            disabled={!name.trim() || unchanged}
          >
            {t('common.save')}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

/* La fiche : de quoi on parle, et ce qu'il y a dedans.
 *
 *  Le nom et la description etaient un bloc separe, trois rangs plus bas que
 *  la card qui decrit la meme chose. Ils se lisent ensemble. L'etat de
 *  fraicheur monte ici aussi : il etait loge dans le bloc « Filtrer », ou
 *  personne ne va. */
function OntologyIdentity({ ontology }: { ontology: Ontology }) {
  return (
    <div className="space-y-3">
      <OntologyNaming ontology={ontology} />
      <OntologyFreshnessNote ontologyId={ontology.id} />
      <OntologyCardPanel ontology={ontology} />
      <OntologyExtractsNote ontology={ontology} />
      <OntologyScanHistory ontology={ontology} />
    </div>
  );
}

/* Combien d'extraits dependent de cette ontologie.
 *
 *  Un fait sur l'ontologie, pas un atelier : la page portait le formulaire de
 *  decoupe entier, alors qu'on est dans « ontologie » et pas dans
 *  « extrait ». Ce qui doit rester ici est qu'en supprimer une ou la
 *  rescanner engage ce qui en a ete tire. La fabrication est dans l'entree de
 *  catalogue qui porte les extraits. */
function OntologyExtractsNote({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const { locale } = useI18n();
  const extracts = useOntologyExtracts(ontology.id);
  const count = extracts.data?.length ?? 0;

  return (
    <p className="px-1 text-xs text-muted-foreground">
      {count === 0 ? t('ontologies.extractsNone') : null}
      {count > 0
        ? t('ontologies.extractsCount', { count: formatNumber(count, locale) })
        : null}{' '}
      <Link to="/catalog/extracts" className="underline underline-offset-2 hover:text-foreground">
        {t('ontologies.extractDeclare')}
      </Link>
    </p>
  );
}

/* Les scans que cette ontologie a traverses.
 *
 *  Un rescan remplace le manifeste et garde l'objet (ADR-043), ce qui est
 *  juste pour l'objet et laisse deux questions sans reponse cinq minutes plus
 *  tard. Un nombre de sujets qui passe de 2 a 31, c'est soit l'etude qui
 *  recrute soit la regle de lecture qui a change - deux causes, deux
 *  reactions opposees - et la plateforme le disait au moment du rescan avant
 *  de jeter ce a quoi elle comparait. Et un extrait gele une liste de
 *  fichiers contre un manifeste que le scan suivant ecrase, donc le n d'une
 *  cohorte devient inexplicable.
 *
 *  Chaque ligne porte la regle qui l'a produite : sans elle, deux
 *  photographies differentes ne se distinguent pas. */
function OntologyScanHistory({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const { locale } = useI18n();
  const mots = motsDuMetier(ontology);
  const history = useOntologyScans(ontology.id);
  const scans = history.data?.scans ?? [];
  if (scans.length === 0) return null;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.scanHistory')}</CardTitle>
          <CardDescription>{t('ontologies.scanHistoryHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent>
        <TableWrapper className="rounded-md border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('ontologies.scanWhen')}</TableHead>
                <TableHead>{t('common.owner')}</TableHead>
                <TableHead className="text-right">{capitaliser(pluriel(mots.sujet))}</TableHead>
                <TableHead className="text-right">{t('ontologyCard.records')}</TableHead>
                <TableHead>{t('ontologies.patternRule')}</TableHead>
                <TableHead>{t('ontologies.scanChanged')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {scans.map((scan) => (
                <TableRow key={scan.id}>
                  <TableCell className="whitespace-nowrap text-xs">
                    {new Date(scan.generatedAt).toLocaleString(locale)}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {scan.generatedBy || '—'}
                  </TableCell>
                  <TableCell className="text-right text-xs tabular-nums">
                    {formatNumber(scan.subjects, locale)}
                  </TableCell>
                  <TableCell className="text-right text-xs tabular-nums">
                    {formatNumber(scan.objects, locale)}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {scan.reading?.source === 'declared'
                      ? t('ontologies.readingDeclared')
                      : t('ontologies.readingDefault')}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    <ScanChange scan={scan} mots={mots} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableWrapper>
      </CardContent>
    </Card>
  );
}

/* Ce qu'un scan a change, et d'abord pourquoi.
 *
 *  La lecture passe avant les chiffres : quand elle a change, tous les autres
 *  ecarts de la ligne s'expliquent par elle jusqu'a preuve du contraire, et
 *  les lire comme une nouvelle sur la donnee est l'erreur que cette colonne
 *  existe pour eviter. */
function ScanChange({
  scan,
  mots,
}: {
  scan: OntologyScan;
  mots: ReturnType<typeof motsDuMetier>;
}) {
  const t = useT();
  const { locale } = useI18n();
  const difference = scan.difference;
  if (!difference) return <span>{t('ontologies.scanFirst')}</span>;

  const ecarts: string[] = [];
  const sujets = difference.currentSubjects - difference.previousSubjects;
  const objets = difference.currentObjects - difference.previousObjects;
  if (sujets !== 0) {
    ecarts.push(
      t('ontologies.scanDelta', {
        count: `${sujets > 0 ? '+' : '−'}${formatNumber(Math.abs(sujets), locale)}`,
        what: pluriel(mots.sujet),
      }),
    );
  }
  if (objets !== 0) {
    ecarts.push(
      t('ontologies.scanDelta', {
        count: `${objets > 0 ? '+' : '−'}${formatNumber(Math.abs(objets), locale)}`,
        what: t('ontologyCard.records').toLowerCase(),
      }),
    );
  }

  return (
    <span className="inline-flex flex-wrap items-center gap-1.5">
      {difference.readingChanged ? (
        <Badge tone="warning" title={difference.currentReading || undefined}>
          {t('ontologies.scanReadingChanged')}
        </Badge>
      ) : null}
      <span>{ecarts.length > 0 ? ecarts.join(', ') : t('ontologies.scanUnchanged')}</span>
    </span>
  );
}

/* Ce qui sert a administrer l'ontologie, plie par defaut.
 *
 *  Rien n'est supprime - la regle de lecture reste modifiable, l'exploration
 *  des objets reste la (ADR-025), la propriete se transfere toujours. Mais
 *  aucune des trois n'est ce qu'on vient faire ici, et les trois ensemble
 *  faisaient les deux tiers de la hauteur de la page. */
function OntologySettings({ ontology }: { ontology: Ontology }) {
  const t = useT();
  return (
    <details className="group rounded-lg border border-border bg-surface-raised">
      <summary className="cursor-pointer list-none px-4 py-3 text-sm font-medium marker:content-none">
        <span className="inline-flex items-center gap-2">
          <ChevronRight className="size-4 transition-transform group-open:rotate-90" aria-hidden />
          {t('ontologies.settings')}
        </span>
        <span className="ml-6 block text-xs font-normal text-muted-foreground">
          {t('ontologies.settingsHint')}
        </span>
      </summary>
      <div className="space-y-3 border-t border-border p-3">
        <OntologyPattern ontology={ontology} />
        <OntologyQuery ontology={ontology} />
        <OntologyOwnership ontology={ontology} />
      </div>
    </details>
  );
}

function OntologyOwnership({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();

  const transfer = useMutation({
    mutationFn: (input: { ownerType: string; ownerId: string }) =>
      ontologiesApi.setOwner(ontology.id, input),
    onSuccess: () => {
      invalidate(qk.ontologies);
      toast.success(t('projects.transferOwnership'));
    },
    onError: (error) => toast.error(error, t('projects.transferOwnership')),
  });

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('projects.transferOwnership')}</CardTitle>
          <CardDescription>{t('ontologies.ownershipHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent>
        <OwnerTransfer
          owner={ontology}
          pending={transfer.isPending}
          onTransfer={(input) => transfer.mutate(input)}
          label={t('projects.transferOwnership')}
        />
      </CardContent>
    </Card>
  );
}

export function OntologyCatalog() {
  // Un bouton qui emet dans le vide est pire qu'un bouton absent : il a
  // l'air casse. La configuration liste deja les extensions declarees.
  const assistantPresent = assistantAvailable();

  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  /* Ce que l'assistant doit avoir sous les yeux pour proposer une lecture.
   *
   *  Il recevait le nom de l'ontologie, sa source et son profil - et rien de la
   *  forme des chemins. Interroge sur SELENA le 2026-10-01 il a repondu "je ne
   *  vois pas les formes de chemins", ce qui etait exact : on ne lui en avait
   *  envoye aucune. La question etait posee a quelqu'un a qui on n'avait pas
   *  montre le dossier.
   *
   *  Des formes, jamais des chemins. Ce sont des metadonnees de contexte de
   *  sante : la lecture se propose sur la structure, et les identifiants n'ont
   *  pas a voyager (ADR-040). */

  const ontologies = useOntologies();
  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [mountedOntology, setMountedOntology] = React.useState<Ontology | null>(null);
  const selected = ontologies.data?.find((ontology) => ontology.id === selectedId) ?? null;

  /* Demander ce que la suppression emporte, avant de la proposer.
   *
   *  Le 2026-10-01 quelqu'un a supprime une ontologie pour la rescanner -
   *  l'habitude d'avant que le rescan rafraichisse sur place - et un extrait
   *  de 4 019 fichiers est parti avec elle. La confirmation disait "supprimer
   *  l'ontologie" et rien d'autre.
   *
   *  Quand il y a quelque chose a perdre, on demande aussi le nom : taper un
   *  nom est le temps d'arret qui manquait. */
  const demanderSuppression = async (ontology: Ontology) => {
    let cout: Awaited<ReturnType<typeof deletionCostApi.ontology>> | null;
    try {
      cout = await deletionCostApi.ontology(ontology.id);
    } catch {
      // Ne pas savoir ce que ca coute n'empeche pas de supprimer : on le dit,
      // et on demande le nom par precaution.
      cout = null;
    }
    const extraits = cout?.extracts ?? 0;
    ask({
      title: t('ontologies.deleteTitle'),
      description:
        extraits > 0
          ? t('ontologies.deleteWarningExtracts', { count: String(extraits) })
          : cout === null
            ? t('ontologies.deleteWarningUnknown')
            : t('ontologies.deleteWarning'),
      confirmLabel: t('common.delete'),
      destructive: true,
      confirmationValue: extraits > 0 || cout === null ? ontology.name : undefined,
      onConfirm: () => remove.mutateAsync(ontology.id),
    });
  };

  const remove = useMutation({
    mutationFn: (ontologyId: string) => ontologiesApi.remove(ontologyId),
    onSuccess: () => {
      invalidate(qk.ontologies);
      setSelectedId(null);
    },
    onError: (error) => toast.error(error, t('ontologies.deleteTitle')),
  });

  const columns: Column<Ontology>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (ontology) => ontology.name,
      searchValue: (ontology) => `${ontology.name} ${ontology.description}`,
      cell: (ontology) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{ontology.name}</p>
          {ontology.description ? (
            <p className="truncate text-xs text-muted-foreground">{ontology.description}</p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'source',
      header: t('ontologies.source'),
      cell: (ontology) => (
        <span className="flex items-center gap-1.5">
          <Badge tone="outline">{ontology.sourceType || '—'}</Badge>
          <span className="truncate text-xs text-muted-foreground">{ontology.sourceName}</span>
        </span>
      ),
    },
    {
      id: 'owner',
      header: t('common.owner'),
      sortValue: (ontology) => ontology.ownerName || ontology.ownerId,
      cell: (ontology) => <ResourceOwner owner={ontology} />,
    },
    {
      id: 'profile',
      header: t('ontologies.reading'),
      /* La regle qui a pris CETTE photographie, pas le nom d'un profil.
       *
       *  C'est la decision d'ADR-040 : une ontologie porte la description avec
       *  laquelle elle a ete construite, et non une etiquette pointant vers
       *  quelque chose qui a pu changer depuis. Le panneau « Motif » le faisait
       *  deja ; la liste, qu'on lit en premier, montrait l'etiquette. Depuis
       *  que le profil ne distingue plus deux ontologies, elle ne disait meme
       *  plus de quoi on parle. Le nom reste en repli pour les photographies
       *  prises avant que la regle soit enregistree. */
      /* Un mot par ligne, le meme mot d'une ligne a l'autre.
       *
       *  La colonne parlait trois langues a la fois : « Par defaut » sur deux
       *  lignes, « level 1 = patient, level 2 = visite » sur une troisieme -
       *  la phrase positionnelle du serveur, en anglais, dans une cellule -
       *  et « health-file-path-v1 » sur une quatrieme, parce qu'une
       *  photographie prise avant que la regle soit enregistree retombait sur
       *  l'identifiant du profil. Trois vocabulaires pour une colonne dont le
       *  seul travail est de distinguer deux lignes.
       *
       *  Ce qu'elle doit dire tient en un mot, et c'est une question de
       *  qualite : une lecture declaree a ete confirmee par quelqu'un qui
       *  connait l'etude, une lecture par defaut est une devinette, et les
       *  chiffres de la ligne valent ce que vaut la lecture qui les a
       *  produits. Le detail se lit au survol et en entier dans les reglages.
       *
       *  Une photographie sans regle enregistree ne dit « par defaut » pour
       *  rien au monde : on ne sait pas comment elle a ete lue, et l'ecrire
       *  serait affirmer un fait que personne n'a verifie. */
      cell: (ontology) => {
        const regle = (ontology.manifest as ManifestLu | undefined)?.readingRule;
        if (!regle?.source) {
          return (
            <span className="text-xs text-muted-foreground" title={t('ontologies.readingUnrecordedHint')}>
              {t('ontologies.readingUnrecorded')}
            </span>
          );
        }
        return (
          <span className="text-xs text-muted-foreground" title={regleLisible(ontology, t)}>
            {regle.source === 'declared'
              ? t('ontologies.readingDeclared')
              : t('ontologies.readingDefault')}
          </span>
        );
      },
    },
    /* Pas de colonne « Projets », et c'est la meme raison que le statut.
     *
     *  Elle affichait « — » sur presque chaque ligne, et un nom tronque sur
     *  les autres. Ce qu'elle voulait dire - « quelque chose depend-il de
     *  ceci ? » - compte a un seul moment, celui ou on s'apprete a supprimer
     *  ou a rescanner, et c'est deja la que la plateforme le dit : le dialogue
     *  de suppression annonce son cout, et la fiche compte les extraits qui en
     *  sont tires. Rattacher ou detacher reste dans le menu de la ligne.
     *
     *  Une colonne qui pose une question au mauvais moment prend de la largeur
     *  a celles qui repondent a la bonne. */
    /* Pas de colonne de statut, et c'est delibere.
     *
     *  Une ontologie n'est pas une charge : elle existe, elle ne tourne pas.
     *  Le domaine n'ecrit jamais que "active", et le badge de statut - ecrit
     *  pour un pod, un job, un workspace - mappe "active" sur "En marche".
     *  La colonne affichait donc la meme chose sur chaque ligne, dans un
     *  vocabulaire qui ment sur la nature de l'objet.
     *
     *  Ce qui dit vraiment quelque chose sur une ontologie - sa fraicheur
     *  par rapport a la source, sa couverture par modalite, ce qu'elle n'a
     *  pas reconnu - a ses propres cartes sous la ligne. */
    {
      id: 'updatedAt',
      header: t('common.updatedAt'),
      sortValue: (ontology) => ontology.updatedAt,
      cell: (ontology) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(ontology.updatedAt, locale)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader title={t('ontologies.title')} description={t('ontologies.subtitle')} />

      <OntologyScan />

      <Card>
        <DataTable
          data={ontologies.data}
          columns={columns}
          rowKey={(ontology) => ontology.id}
          isLoading={ontologies.isLoading}
          isError={ontologies.isError}
          error={ontologies.error}
          onRetry={() => void ontologies.refetch()}
          onRowClick={(ontology) => setSelectedId(ontology.id)}
          defaultSort={{ columnId: 'updatedAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={Network}
              title={t('ontologies.empty')}
              description={t('ontologies.emptyHint')}
            />
          }
          rowActions={(ontology) => (
            <>
              {/* Des actions sur la ligne, et rien d'autre.
                *
                *  Il y en avait cinq, dont deux qui appelaient setSelectedId -
                *  le meme clic que sur la ligne elle-meme, et le meme clic l'un
                *  que l'autre, sous deux noms differents. « Ce que decrit cette
                *  ontologie » et « Filtrer » ne faisaient pas ce qu'ils
                *  disaient : ils ouvraient la fiche, ou ces deux choses se
                *  trouvent parmi d'autres. Un menu qui nomme des blocs deja
                *  visibles dessous n'est pas un menu, c'est un sommaire - et un
                *  sommaire dont deux entrees menent au meme endroit.
                *
                *  Reste ce qu'on ne peut pas faire en cliquant la ligne :
                *  rattacher a un projet, demander une lecture a l'assistant,
                *  supprimer. */}
              <DropdownMenuItem onSelect={() => setMountedOntology(ontology)}>
                <FolderOpen aria-hidden />
                {t('mounts.title')}
              </DropdownMenuItem>
              {/* Un bouton dedie, et pas le lanceur generique.
                *
                *  L'assistant porte une consigne par surface : celle-ci lui
                *  donne les formes de chemins et lui demande une lecture, sans
                *  aucun outil pour l'appliquer (ADR-040). Ouvrir la meme
                *  conversation depuis le lanceur flottant donnerait l'autre
                *  consigne, celle qui sait chercher un ticket et pas lire une
                *  arborescence. */}
              {assistantPresent ? (
                <DropdownMenuItem
                  onSelect={() => {
                    demanderUneLecture(
                      ontology,
                      undefined,
                      t('ontologies.describePrompt', { name: ontology.name }),
                    );
                    toast.success(t('ontologies.describeSent'), t('ontologies.describe'));
                  }}
                >
                  <MessageCircle aria-hidden />
                  {t('ontologies.describe')}
                </DropdownMenuItem>
              ) : null}
              <DropdownMenuItem
                destructive
                onSelect={() => void demanderSuppression(ontology)}
              >
                <Trash2 aria-hidden />
                {t('common.delete')}
              </DropdownMenuItem>
            </>
          )}
        />
      </Card>

      <ProjectMountsSheet
        kind="ontology"
        resourceId={mountedOntology?.id ?? null}
        resourceName={mountedOntology?.name}
        open={Boolean(mountedOntology)}
        onOpenChange={(open) => {
          if (!open) setMountedOntology(null);
        }}
      />
      {/* Deux blocs, puis un tiroir.
        *
        *  Il y en avait sept, empiles : la card, les formes de chemins, le
        *  renommage, un filtre, un tableau de couverture, les extraits, la
        *  propriete. Quelqu'un qui ouvre une ontologie pose trois questions -
        *  c'est quoi, il y a quoi dedans, comment j'en prends un bout - et la
        *  page repondait a la troisieme en sixieme position, apres deux blocs
        *  de diagnostic et un tableau qui disait « rien a signaler ».
        *
        *  Ce qui reste a l'ouverture repond a ces trois questions. Le reste
        *  est de l'administration : la regle de lecture, l'exploration des
        *  objets, la propriete. Toujours la, jamais en travers. */}
      {selected ? (
        <>
          <OntologyIdentity ontology={selected} />
          <OntologyCoverage ontology={selected} />
          <OntologySettings ontology={selected} />
        </>
      ) : null}
      {dialog}
    </div>
  );
}
