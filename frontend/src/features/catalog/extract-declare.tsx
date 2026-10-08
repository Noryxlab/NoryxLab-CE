import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { useToast } from '@/components/ui/toast';
import { ontologiesApi } from '@/lib/api/endpoints';
import { qk, useInvalidate, useOntologyCompleteness } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatNumber } from '@/lib/format';
import type { Ontology } from '@/lib/api/types';
import { capitaliser, motsDuMetier, pluriel } from './ontology-words';
import { reordonner } from './extract-layout';

/* Declarer un extrait.
 *
 *  Le formulaire vivait sur la page d'une ontologie, qui est la page de
 *  l'objet qui decrit - pas de l'objet qu'on emporte. Il y a une entree de
 *  catalogue « Extraits », et c'est la qu'on en fabrique un : l'ontologie
 *  devient alors une chose qu'on choisit, comme on choisit un dataset avant
 *  de le scanner.
 *
 *  Ce que la page d'ontologie garde est un fait sur l'ontologie - « trois
 *  extraits en sont tires » - et non un atelier. */
export function ExtractDeclareForm({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const mots = motsDuMetier(ontology);

  const [name, setName] = React.useState('');
  const [subjects, setSubjects] = React.useState('');
  /* Les modalites se choisissent, elles ne se tapent plus.
   *
   *  Le champ etait libre, avec un exemple en filigrane, et l'ecran ne disait
   *  nulle part comment le scan avait nomme les categories de CETTE
   *  ontologie. Le 2026-10-01, un extrait declare "Selena-extract-modality"
   *  est reparti avec un filtre vide - donc l'ontologie entiere - et rien a
   *  l'ecran ne contredisait son nom. Un n faux se decouvre trois mois plus
   *  tard, dans un article. */
  const [chosen, setChosen] = React.useState<string[]>([]);
  /* Le genre de fichier, a cote des categories.
   *
   *  C'est la demande d'Essilor du 2026-10-08, et les chiffres la justifient
   *  seuls : dans l'ANTERION de PREMYOM1000, les mesures sont 349 CSV pour
   *  37 Mo et les images 20 115 fichiers pour 41 Go. Quelqu'un qui veut les
   *  chiffres devait emporter les images, parce que la categorie etait la
   *  plus petite chose qu'on pouvait demander. */
  const [kinds, setKinds] = React.useState<string[]>([]);
  /* La table de mesure : l'axe sous la categorie.
   *
   *  C'est ce que quelqu'un veut dire en demandant « un niveau de plus ».
   *  Dans l'ANTERION de PREMYOM1000, le quatrieme segment de chemin contient
   *  trois noms de dossier et 349 noms de fichiers, alors que les tables sont
   *  sept, chacune chez les trente patients. */
  const [tables, setTables] = React.useState<string[]>([]);
  /* La disposition : l'ordre des niveaux de l'arbre monte.
   *
   *  La selection dit quels fichiers, la disposition dit comment ils sont
   *  ranges pour travailler. Par sujet on repond a « que possede ce
   *  patient », par categorie a « montre-moi tous mes scans de cornee » -
   *  deux questions, un seul ensemble de fichiers. */
  /* La disposition se construit niveau par niveau.
   *
   *  C'etait une liste de trois permutations sur six, choisies parce que
   *  l'ecran les avait ecrites - et un menu unique « Sujet > visite >
   *  modalite » ne montre pas qu'on decide d'un ordre, il a l'air d'un
   *  reglage a prendre ou a laisser. Trois choix successifs disent ce qu'ils
   *  sont : le premier dossier, puis le second, puis le troisieme.
   *
   *  On peut s'arreter avant trois. Sur une etude ou chaque patient n'a
   *  qu'une visite, le repertoire de visite ne sert qu'a etre traverse, et
   *  « patient / modalite » est l'arbre qu'on veut vraiment. Le serveur
   *  refuse si deux fichiers s'y retrouveraient au meme endroit. */
  const [niveaux, setNiveaux] = React.useState<string[]>(['subject', 'visit', 'modality']);

  const coverage = useOntologyCompleteness(ontology.id);
  const available = coverage.data?.modalities ?? [];
  const genres = coverage.data?.formats ?? [];
  const mesures = coverage.data?.tables ?? [];

  // Repartir de zero quand on change d'ontologie, sinon le formulaire propose
  // de decouper la nouvelle avec les categories de l'ancienne.
  React.useEffect(() => {
    setName('');
    setSubjects('');
    setChosen([]);
    setKinds([]);
    setTables([]);
    setNiveaux(['subject', 'visit', 'modality']);
  }, [ontology.id]);

  const bascule = (liste: string[], nom: string) =>
    liste.includes(nom) ? liste.filter((item) => item !== nom) : [...liste, nom];
  const toggle = (nom: string) => setChosen((current) => bascule(current, nom));
  const toggleKind = (nom: string) => setKinds((current) => bascule(current, nom));
  const toggleTable = (nom: string) => setTables((current) => bascule(current, nom));

  const asList = (raw: string) =>
    raw
      .split(',')
      .map((value) => value.trim())
      .filter(Boolean);

  const done = (count: number) => {
    toast.success(t('ontologies.extractCreated', { count: formatNumber(count, locale) }));
    invalidate(qk.ontologyExtracts(ontology.id));
    invalidate(qk.extracts);
  };

  const create = useMutation({
    mutationFn: () =>
      ontologiesApi.createExtract(ontology.id, {
        name: name.trim(),
        // Left to the server when the ontology belongs to exactly one project;
        // it refuses with an explanation when the answer is ambiguous, which
        // is better than filing the extract under a project that will never
        // mount it.
        modalities: chosen,
        formats: kinds,
        tables: tables,
        subjects: asList(subjects),
        layout: niveaux,
      }),
    onSuccess: (created) => {
      setName('');
      setChosen([]);
      setKinds([]);
      setTables([]);
      setSubjects('');
      setNiveaux(['subject', 'visit', 'modality']);
      done(created.objectCount);
    },
    onError: (error) => toast.error(error, t('ontologies.extractCreate')),
  });

  /* L'extrait maximal, c'est l'ontologie entiere.
   *
   *  Mecaniquement c'etait deja vrai - un filtre vide prend tout - mais ce
   *  n'etait nomme nulle part, donc « et si je veux tout ? » restait une
   *  question. Une ontologie ne se monte pas : elle decrit, elle ne contient
   *  pas. Ce bouton est la facon de la monter quand meme, en disant ce que ca
   *  veut dire. */
  const createWhole = useMutation({
    mutationFn: () =>
      ontologiesApi.createExtract(ontology.id, {
        name: `${ontology.name} — ${t('ontologies.extractWholeSuffix')}`,
        description: t('ontologies.extractWholeHint'),
        modalities: [],
        formats: [],
        tables: [],
        subjects: [],
        layout: ['subject', 'visit', 'modality'],
      }),
    onSuccess: (created) => done(created.objectCount),
    onError: (error) => toast.error(error, t('ontologies.extractWhole')),
  });

  /* Ce que la selection prendra, dit avant de declarer.
   *
   *  Le formulaire ne disait nulle part combien de fichiers il allait geler.
   *  Un extrait nomme « SELENA-modality » est reparti un jour avec un filtre
   *  vide - donc l'ontologie entiere - et rien a l'ecran ne contredisait son
   *  nom. Un n faux se decouvre trois mois plus tard, dans un article. */
  const totalFichiers = available.reduce((total, item) => total + item.objects, 0);
  const prisParCategorie =
    chosen.length === 0
      ? totalFichiers
      : available
          .filter((item) => chosen.includes(item.name))
          .reduce((total, item) => total + item.objects, 0);
  const filtreEntites = asList(subjects).length > 0;
  // Le filtre par genre croise celui des categories, donc on ne peut pas
  // additionner : on annonce une borne haute et on le dit.
  const prisParGenre =
    kinds.length === 0
      ? null
      : genres.filter((genre) => kinds.includes(genre.name)).reduce((t, g) => t + g.objects, 0);
  const prisParTable =
    tables.length === 0
      ? null
      : mesures.filter((m) => tables.includes(m.name)).reduce((t, m) => t + m.objects, 0);
  const borneHaute = filtreEntites || prisParGenre !== null || prisParTable !== null;
  const annonce = [prisParCategorie, prisParGenre, prisParTable]
    .filter((valeur): valeur is number => valeur !== null)
    .reduce((plus_petit, valeur) => Math.min(plus_petit, valeur), prisParCategorie);

  const lu = ontology.manifest as ManifestExtrait | undefined;
  const resume = lu?.summary;
  // La date est a la racine du manifeste, pas dans son resume : c'est le
  // moment ou la photographie a ete prise, et un extrait se raisonne contre
  // elle.
  const prisLe = lu?.generatedAt ? new Date(lu.generatedAt) : null;

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        if (name.trim()) create.mutate();
      }}
      className="space-y-5"
    >
      {/* De quelle photographie on decoupe.
        *
        *  Un extrait gele une liste de fichiers prise dans UNE lecture a UN
        *  moment : sans le dire, « 1 934 fichiers » est un chiffre sans
        *  provenance, et les mots du formulaire - entite, periode - sortent
        *  de nulle part. Ils viennent de la, et la ligne le dit. */}
      <p className="text-xs text-muted-foreground">
        {t('ontologies.extractFrom', {
          name: ontology.name,
          when: prisLe ? prisLe.toLocaleDateString(locale, { dateStyle: 'long' }) : '—',
        })}{' '}
        <span className="tabular-nums">
          {t('ontologies.extractFromFigures', {
            entities: formatNumber(resume?.subjects ?? 0, locale),
            entityName: pluriel(mots.sujet),
            files: formatNumber(totalFichiers, locale),
            categories: formatNumber(available.length, locale),
            categoryName: pluriel(mots.modalite),
          })}
        </span>
      </p>

      <Field label={t('common.name')} className="sm:max-w-sm">
        <Input value={name} onChange={(event) => setName(event.target.value)} />
      </Field>

      {/* Deux questions, deux blocs, dans l'ordre ou on se les pose. */}
      <Etape titre={t('ontologies.extractWhat')} sous={t('ontologies.extractWhatHint')}>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field
            label={capitaliser(pluriel(mots.modalite))}
            description={t('ontologies.extractPickCategories', {
              categories: pluriel(mots.modalite),
            })}
          >
            <div className="flex flex-wrap gap-1.5">
              {available.length === 0 ? (
                <span className="text-xs text-muted-foreground">
                  {t('ontologies.extractModalitiesUnknown')}
                </span>
              ) : (
                available.map((item) => (
                  <Button
                    key={item.name}
                    type="button"
                    size="sm"
                    variant={chosen.includes(item.name) ? 'primary' : 'secondary'}
                    onClick={() => toggle(item.name)}
                    aria-pressed={chosen.includes(item.name)}
                  >
                    {item.name}
                    <span className="opacity-70 tabular-nums">
                      {formatNumber(item.objects, locale)}
                    </span>
                  </Button>
                ))
              )}
            </div>
          </Field>

          {/* Le genre de fichier, quand la photographie l'a compte.
            *  Vide, on ne montre rien : une vieille photographie n'a pas
            *  mesure les genres, et proposer un filtre sans valeurs serait
            *  proposer un filtre qui ne filtre rien. */}
          {genres.length > 0 ? (
            <Field
              label={t('ontologies.extractKinds')}
              description={
                kinds.length === 0
                  ? t('ontologies.extractKindsAll')
                  : t('ontologies.extractKindsChosen')
              }
            >
              <div className="flex flex-wrap gap-1.5">
                {genres.map((genre) => (
                  <Button
                    key={genre.name}
                    type="button"
                    size="sm"
                    variant={kinds.includes(genre.name) ? 'primary' : 'secondary'}
                    onClick={() => toggleKind(genre.name)}
                    aria-pressed={kinds.includes(genre.name)}
                    title={t('ontologies.extractKindWeight', {
                      objects: formatNumber(genre.objects, locale),
                      size: formatBytes(genre.totalBytes, locale),
                    })}
                  >
                    {genre.name}
                    <span className="opacity-70 tabular-nums">
                      {formatBytes(genre.totalBytes, locale)}
                    </span>
                  </Button>
                ))}
              </div>
            </Field>
          ) : null}

          {/* Les tables de mesure, quand la photographie en a trouve. */}
          {mesures.length > 0 ? (
            <Field
              label={t('ontologies.extractTables')}
              description={
                tables.length === 0
                  ? t('ontologies.extractTablesAll')
                  : t('ontologies.extractTablesChosen')
              }
            >
              <div className="flex flex-wrap gap-1.5">
                {mesures.map((mesure) => (
                  <Button
                    key={mesure.name}
                    type="button"
                    size="sm"
                    variant={tables.includes(mesure.name) ? 'primary' : 'secondary'}
                    onClick={() => toggleTable(mesure.name)}
                    aria-pressed={tables.includes(mesure.name)}
                  >
                    {mesure.name}
                    <span className="opacity-70 tabular-nums">
                      {formatNumber(mesure.objects, locale)}
                    </span>
                  </Button>
                ))}
              </div>
            </Field>
          ) : null}

          <Field
            label={capitaliser(pluriel(mots.sujet))}
            description={t('ontologies.extractPickEntities', { entities: pluriel(mots.sujet) })}
          >
            {/* Un exemple pris dans cette ontologie, jamais une autre : le
              *  champ affichait « PREMYOM1000-001 » sur la page de SELENA,
              *  code en dur, et rien ne disait que c'etait un exemple. */}
            <Input
              value={subjects}
              onChange={(event) => setSubjects(event.target.value)}
              placeholder={premiereEntite(ontology)}
            />
          </Field>
        </div>

        <p className="text-xs font-medium">
          {borneHaute
            ? t('ontologies.extractTakesAtMost', {
                files: formatNumber(annonce, locale),
                total: formatNumber(totalFichiers, locale),
              })
            : t('ontologies.extractTakes', {
                files: formatNumber(annonce, locale),
                total: formatNumber(totalFichiers, locale),
              })}
        </p>
      </Etape>

      <Etape titre={t('ontologies.extractHow')} sous={t('ontologies.extractHowHint')}>
        <LayoutBuilder
          ontology={ontology}
          niveaux={niveaux}
          onChange={setNiveaux}
          nomExtrait={name.trim()}
        />
      </Etape>

      <div className="flex flex-wrap gap-2">
        <Button type="submit" variant="primary" loading={create.isPending} disabled={!name.trim()}>
          {t('ontologies.extractCreate')}
        </Button>
        <Button
          type="button"
          variant="secondary"
          loading={createWhole.isPending}
          onClick={() => createWhole.mutate()}
          title={t('ontologies.extractWholeHint')}
        >
          {t('ontologies.extractWhole')}
        </Button>
      </div>
    </form>
  );
}

/** Un identifiant que cette ontologie porte vraiment, pour servir d'exemple. */
function premiereEntite(ontology: Ontology): string | undefined {
  const sujets = (ontology.manifest as ManifestExtrait | undefined)?.subjects;
  return sujets?.[0]?.id?.trim() || undefined;
}

/* Trois choix successifs, et l'arbre qu'ils produisent.
 *
 *  L'apercu n'est pas un ornement : « sujet, puis visite, puis modalite » est
 *  une phrase, et « SELENA-01-001 / 18 fev. 2026 / ANTERION / ... » est un
 *  chemin qu'on reconnait. Le second se verifie d'un coup d'oeil, le premier
 *  se relit deux fois. */
const TOUS: string[] = ['subject', 'visit', 'modality'];

function LayoutBuilder({
  ontology,
  niveaux,
  onChange,
  nomExtrait,
}: {
  ontology: Ontology;
  niveaux: string[];
  onChange: (niveaux: string[]) => void;
  nomExtrait: string;
}) {
  const t = useT();
  const mots = motsDuMetier(ontology);
  const nomDuNiveau: Record<string, string> = {
    subject: mots.sujet,
    visit: mots.visite,
    modality: mots.modalite,
  };
  const exemple: Record<string, string> = {
    subject: premiereEntite(ontology) ?? mots.sujet.toUpperCase(),
    visit: premiereVisite(ontology) ?? mots.visite.toUpperCase(),
    modality: premiereCategorie(ontology) ?? mots.modalite.toUpperCase(),
  };

  /* Les trois niveaux sont toujours proposes, a chaque position.
   *
   *  Ils ne l'etaient pas : chaque menu n'offrait que les niveaux encore
   *  libres, ce qui parait prudent et ne marche pas du tout. Avec les trois
   *  niveaux places, aucun n'est libre, donc chaque menu proposait exactement
   *  sa propre valeur - trois listes a un seul choix. Impossible de trier par
   *  date : le seul geste qui en avait besoin etait celui qu'on empechait. */
  /* Le mot, et une valeur que ce niveau porte vraiment.
   *
   *  « Entite » et « Periode » ne disent rien a quelqu'un qui n'a pas ecrit
   *  le scan : ce sont des categories grammaticales, pas des choses qu'on
   *  reconnait. « Entite — SELENA-01-001 » se comprend sans explication, et
   *  l'exemple sort de cette ontologie-ci. */
  const choix = TOUS.map((level) => ({
    value: level,
    label: capitaliser(nomDuNiveau[level] ?? level),
    hint: exemple[level],
  }));


  const changer = (rang: number, level: string) => onChange(reordonner(niveaux, rang, level));

  const retirer = (rang: number) => onChange(niveaux.filter((_, index) => index !== rang));

  const suivant = TOUS.find((level) => !niveaux.includes(level));

  return (
    <div className="space-y-2">
      {niveaux.map((level, rang) => (
        <div key={`${level}-${rang}`} className="flex items-center gap-2">
          <span className="w-16 shrink-0 text-xs text-muted-foreground">
            {t('ontologies.layoutLevel', { rank: String(rang + 1) })}
          </span>
          <Select
            value={level}
            onValueChange={(value) => changer(rang, value)}
            options={choix}
            className="flex-1"
          />
          {/* Retirer n'est propose que sur le dernier : un trou au milieu
            *  n'est pas un arbre, c'est le meme arbre plus court. */}
          {rang === niveaux.length - 1 && niveaux.length > 1 ? (
            <Button type="button" variant="ghost" size="sm" onClick={() => retirer(rang)}>
              {t('ontologies.layoutDrop')}
            </Button>
          ) : null}
        </div>
      ))}

      {suivant ? (
        <Button type="button" variant="ghost" size="sm" onClick={() => onChange([...niveaux, suivant])}>
          {t('ontologies.layoutAdd', { name: nomDuNiveau[suivant] ?? suivant })}
        </Button>
      ) : null}

      {/* Le chemin entier, tel qu'il existera dans le workspace.
        *
        *  L'apercu montrait les trois segments sans dire ou ils atterrissent,
        *  donc il ressemblait a un resume de la regle plutot qu'a un endroit.
        *  Avec /extracts et le nom devant, c'est un chemin qu'on peut aller
        *  taper dans un terminal. */}
      <p className="break-all font-mono text-xs">
        <span className="text-muted-foreground">/extracts/</span>
        <span>{nomExtrait || t('ontologies.layoutUnnamed')}</span>
        {niveaux.map((level) => (
          <span key={level}>
            <span className="text-muted-foreground">/</span>
            {exemple[level] ?? level}
          </span>
        ))}
        <span className="text-muted-foreground">/{t('ontologies.layoutLeaf')}</span>
      </p>
      {niveaux.length < 3 ? (
        <p className="text-xs text-muted-foreground">{t('ontologies.layoutShortWarning')}</p>
      ) : null}
    </div>
  );
}

function premiereVisite(ontology: Ontology): string | undefined {
  const sujets = (ontology.manifest as ManifestExtrait | undefined)?.subjects;
  return sujets?.[0]?.visits?.[0]?.date?.trim() || undefined;
}

function premiereCategorie(ontology: Ontology): string | undefined {
  const sujets = (ontology.manifest as ManifestExtrait | undefined)?.subjects;
  return sujets?.[0]?.visits?.[0]?.modalities?.[0]?.name?.trim() || undefined;
}

type ManifestExtrait = {
  subjects?: { id?: string; visits?: { date?: string; modalities?: { name?: string }[] }[] }[];
  summary?: { subjects?: number; objects?: number };
  generatedAt?: string;
};

/* Une etape du formulaire : un titre qui dit la question, et sa reponse.
 *
 *  Les champs etaient poses cote a cote sans rien qui les regroupe, donc
 *  « Categories » n'avait pas de sens : ni « prendre seulement celles-ci »
 *  ni « ranger par celles-ci », juste un mot au-dessus de cinq boutons. Deux
 *  questions se posent en declarant un extrait - ce qu'on emporte, comment
 *  on le retrouve - et chacune a maintenant son titre. */
function Etape({
  titre,
  sous,
  children,
}: {
  titre: string;
  sous: string;
  children: React.ReactNode;
}) {
  return (
    <section className="space-y-3 rounded-md border border-border p-3">
      <div>
        <h4 className="text-sm font-medium">{titre}</h4>
        <p className="text-xs text-muted-foreground">{sous}</p>
      </div>
      {children}
    </section>
  );
}
