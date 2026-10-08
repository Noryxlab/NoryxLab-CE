import * as React from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
import { ontologyCardApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/api/queries';
import type { Ontology, OntologyCard } from '@/lib/api/types';
import { useI18n, useT } from '@/lib/i18n';
import { capitaliser, dateDeVisite, motsDuMetier, pluriel } from './ontology-words';

/** Ce qu'une ontologie dit du sens des donnees, et ce que la plateforme a
 *  mesure a cote (ADR-047).
 *
 *  Un seul champ libre. La premiere version en avait quatorze etiquetes -
 *  etude, population, criteres d'inclusion, consentement... - plus six chiffres
 *  verifiables, et deux choses clochaient. Un formulaire aussi long est un
 *  formulaire que personne ne remplit : il aurait recolte quatorze champs vides
 *  au lieu d'un paragraphe que quelqu'un ecrit vraiment. Et la moitie de ces
 *  etiquettes etait le vocabulaire d'un hopital, sur une plateforme vendue
 *  aussi a des banques et a des assureurs - « population » et « visites » ne
 *  veulent rien dire sur une table d'operations, et une demo qui les affiche
 *  perd la salle.
 *
 *  Un paragraphe est neutre par construction. C'est aussi ce que les gens
 *  ecrivent quand on leur demande ce qu'est un jeu de donnees.
 *
 *  Jamais seul, en revanche : la confiance n'exclut pas le controle, donc les
 *  chiffres que la plateforme a pris elle-meme voyagent avec la declaration.
 *  Sans verdict - une prose ne se verifie pas - mais cote a cote, et le lecteur
 *  juge. */
export function OntologyCardPanel({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const client = useQueryClient();
  const [editing, setEditing] = React.useState(false);
  const [draft, setDraft] = React.useState('');

  const card = useQuery({
    queryKey: qk.ontologyCard(ontology.id),
    queryFn: () => ontologyCardApi.get(ontology.id),
    enabled: Boolean(ontology.id),
  });

  const save = useMutation({
    mutationFn: () => ontologyCardApi.set(ontology.id, { text: draft }),
    onSuccess: () => {
      setEditing(false);
      void client.invalidateQueries({ queryKey: qk.ontologyCard(ontology.id) });
    },
  });

  const content = card.data?.card ?? null;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologyCard.title')}</CardTitle>
          <CardDescription>{t('ontologyCard.hint')}</CardDescription>
        </CardHeaderText>
        {!editing ? (
          <Button
            variant="secondary"
            disabled={card.isLoading}
            onClick={() => {
              setDraft(content?.text ?? '');
              setEditing(true);
            }}
          >
            {card.data?.declared ? t('ontologyCard.edit') : t('ontologyCard.declare')}
          </Button>
        ) : null}
      </CardHeader>

      <CardContent className="space-y-5">
        {editing ? (
          <div className="space-y-3">
            <Textarea
              className="min-h-40"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder={t('ontologyCard.placeholder')}
            />
            <div className="flex gap-2">
              <Button variant="primary" loading={save.isPending} onClick={() => save.mutate()}>
                {t('common.save')}
              </Button>
              <Button variant="ghost" onClick={() => setEditing(false)}>
                {t('common.cancel')}
              </Button>
            </div>
          </div>
        ) : (
          <>
            {/* Vide est un etat legitime et se montre comme vide : un champ sans
                reponse est une information, et le remplir d'une supposition est
                precisement ce qu'on veut eviter. */}
            {card.data?.declared && content ? (
              <div className="space-y-2">
                <p className="whitespace-pre-line text-sm">{content.text}</p>
                <p className="text-xs text-muted-foreground">
                  {t('ontologyCard.versionLine', {
                    version: String(content.version),
                    who: content.declaredBy ?? '—',
                  })}
                </p>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">{t('ontologyCard.undeclared')}</p>
            )}

            <Separator />
            <Measured payload={card.data} ontology={ontology} />
          </>
        )}
      </CardContent>
    </Card>
  );
}

/** Ce que la plateforme a compte elle-meme, et si quelque chose a deja regarde
 *  dans les fichiers. Deux questions differentes : compter des objets ne dit
 *  rien de ce qu'ils contiennent. */
function Measured({
  payload,
  ontology,
}: {
  payload: OntologyCard | undefined;
  ontology: Ontology;
}) {
  const t = useT();
  const { locale } = useI18n();
  if (!payload) return null;
  const { measured, structure } = payload;
  // Les mots de cette ontologie, pas ceux de la plateforme : la card disait
  // « Entites » pendant que la regle de lecture disait « patient ».
  const mots = motsDuMetier(ontology);
  const periode = measured?.firstVisit
    ? [dateDeVisite(measured.firstVisit, locale), dateDeVisite(measured.lastVisit, locale) || '…']
    : null;

  return (
    <div className="space-y-3">
      <h4 className="text-sm font-medium">{t('ontologyCard.measured')}</h4>

      <dl className="flex flex-wrap gap-x-6 gap-y-2 text-xs">
        <Figure label={capitaliser(pluriel(mots.sujet))} value={measured?.subjects} />
        <Figure label={t('ontologyCard.records')} value={measured?.objects} />
        {measured?.modalities?.length ? (
          <Figure
            label={capitaliser(pluriel(mots.modalite))}
            value={measured.modalities.join(', ')}
          />
        ) : null}
        {periode ? (
          <Figure
            label={capitaliser(mots.visite)}
            value={periode[0] === periode[1] ? periode[0] : `${periode[0]} → ${periode[1]}`}
          />
        ) : null}
      </dl>

      {/* Ce que la regle ne couvre pas, dit a cote du total plutot que fondu
          dedans : un fichier hors lecture n'est pas une erreur, c'est une
          chose a savoir avant de citer un n. */}
      {measured?.unreadable ? (
        <p className="text-xs text-muted-foreground">
          {t('ontologyCard.unreadable', { count: measured.unreadable })}
        </p>
      ) : null}

      <p className="text-xs text-muted-foreground">
        {measured?.method
          ? t('ontologyCard.measuredBy', {
              method: t('ontologyCard.methodPathScan'),
              when: measured.at ? new Date(measured.at).toLocaleString(locale) : '—',
            })
          : t('ontologyCard.neverMeasured')}
      </p>

      {/* Regarder dans les fichiers est un autre acte que lister leurs clefs,
          et son absence est une information : « personne n'a verifie » n'est
          pas « rien n'a ete trouve ». */}
      <div className="flex flex-wrap items-center gap-2">
        {!structure?.ran ? (
          <Badge tone="neutral">{t('ontologyCard.structureNever')}</Badge>
        ) : structure.identifiersPresent ? (
          <>
            <Badge tone="danger">{t('ontologyCard.identifiersFound')}</Badge>
            <span className="font-mono text-xs text-muted-foreground">
              {(structure.present ?? []).join(', ')}
            </span>
          </>
        ) : (
          <Badge tone="success">{t('ontologyCard.identifiersNone')}</Badge>
        )}
        {structure?.ran ? (
          <span className="text-xs text-muted-foreground">
            {t('ontologyCard.structureBy', {
              method: structure.method ?? '—',
              count: String((structure.checked ?? []).length),
              when: structure.at ? new Date(structure.at).toLocaleString() : '—',
            })}
          </span>
        ) : null}
      </div>
    </div>
  );
}

function Figure({ label, value }: { label: string; value?: number | string }) {
  if (value === undefined || value === null || value === '' || value === 0) return null;
  return (
    <div>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-medium tabular-nums">{value}</dd>
    </div>
  );
}
