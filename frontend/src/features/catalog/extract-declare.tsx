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
import { formatNumber } from '@/lib/format';
import type { Ontology } from '@/lib/api/types';
import { motsDuMetier, pluriel } from './ontology-words';

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
  /* La disposition : l'ordre des niveaux de l'arbre monte.
   *
   *  La selection dit quels fichiers, la disposition dit comment ils sont
   *  ranges pour travailler. Par sujet on repond a « que possede ce
   *  patient », par categorie a « montre-moi tous mes scans de cornee » -
   *  deux questions, un seul ensemble de fichiers. */
  const [layout, setLayout] = React.useState('subject,visit,modality');

  const coverage = useOntologyCompleteness(ontology.id);
  const available = coverage.data?.modalities ?? [];

  // Repartir de zero quand on change d'ontologie, sinon le formulaire propose
  // de decouper la nouvelle avec les categories de l'ancienne.
  React.useEffect(() => {
    setName('');
    setSubjects('');
    setChosen([]);
    setLayout('subject,visit,modality');
  }, [ontology.id]);

  const toggle = (nom: string) =>
    setChosen((current) =>
      current.includes(nom) ? current.filter((item) => item !== nom) : [...current, nom],
    );

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
        subjects: asList(subjects),
        layout: layout.split(','),
      }),
    onSuccess: (created) => {
      setName('');
      setChosen([]);
      setSubjects('');
      setLayout('subject,visit,modality');
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
        subjects: [],
        layout: ['subject', 'visit', 'modality'],
      }),
    onSuccess: (created) => done(created.objectCount),
    onError: (error) => toast.error(error, t('ontologies.extractWhole')),
  });

  const chosenObjects = available
    .filter((item) => chosen.includes(item.name))
    .reduce((total, item) => total + item.objects, 0);

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        if (name.trim()) create.mutate();
      }}
      className="grid gap-3 sm:grid-cols-2 lg:grid-cols-[1fr_1.4fr_1fr_1fr] lg:items-start"
    >
      <Field label={t('common.name')}>
        <Input value={name} onChange={(event) => setName(event.target.value)} />
      </Field>

      <Field
        label={t('ontologies.extractModalities')}
        description={
          chosen.length === 0
            ? t('ontologies.extractModalitiesAll')
            : t('ontologies.extractModalitiesChosen', {
                objects: formatNumber(chosenObjects, locale),
              })
        }
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

      <Field label={t('ontologies.extractLayout')} description={t('ontologies.extractLayoutHint')}>
        <Select
          value={layout}
          onValueChange={setLayout}
          options={[
            { value: 'subject,visit,modality', label: t('ontologies.layoutSubjectFirst') },
            { value: 'modality,subject,visit', label: t('ontologies.layoutModalityFirst') },
            { value: 'visit,subject,modality', label: t('ontologies.layoutVisitFirst') },
          ]}
        />
      </Field>

      <Field label={t('ontologies.extractSubjects', { entities: pluriel(mots.sujet) })}>
        {/* Un exemple pris dans cette ontologie, jamais une autre : le champ
          *  affichait « PREMYOM1000-001 » sur la page de SELENA, code en dur,
          *  et rien ne disait que c'etait un exemple. */}
        <Input
          value={subjects}
          onChange={(event) => setSubjects(event.target.value)}
          placeholder={premiereEntite(ontology)}
        />
      </Field>

      <div className="flex flex-wrap gap-2 sm:col-span-2 lg:col-span-4">
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
  const sujets = (ontology.manifest as { subjects?: { id?: string }[] } | undefined)?.subjects;
  return sujets?.[0]?.id?.trim() || undefined;
}
