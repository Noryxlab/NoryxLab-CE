import { describe, expect, it } from 'vitest';
import { lireProposition } from './reading-proposal';

describe('lireProposition', () => {
  it("lit la forme exacte que la consigne impose a l'assistant", () => {
    const propose = lireProposition(
      'Je te propose la lecture suivante : subject: level 1, visit: level 2, modality: level 3.',
    );
    expect(propose).toMatchObject({ subjectLevel: 1, visitLevel: 2, modalityLevel: 3 });
  });

  it('lit les mots du metier quand ils sont la', () => {
    const propose = lireProposition(
      'subject: level 1, visit: level 2, modality: level 3\nnames: patient / visite / modalité',
    );
    expect(propose).toMatchObject({
      subjectName: 'patient',
      visitName: 'visite',
      modalityName: 'modalité',
    });
  });

  // La reponse reelle du 2026-10-08 sur SELENA : les niveaux dans la forme
  // demandee, les mots en prose. Une proposition a moitie structuree reste
  // utile, et les mots se tapent a la main.
  it('accepte une reponse entiere, collee telle quelle, sans ligne names', () => {
    const propose = lireProposition(`Je te propose la lecture suivante : subject: level 1, visit: level 2, modality: level 3.

En termes métier, cela correspond à :
Niveau 1 : Patient
Niveau 2 : Visite
Niveau 3 : Modalité`);
    expect(propose).toMatchObject({ subjectLevel: 1, visitLevel: 2, modalityLevel: 3 });
    expect(propose?.subjectName).toBeUndefined();
  });

  // Un niveau que l'assistant dit ne pas savoir ne doit pas arriver a zero :
  // zero est une regle, et une regle accidentelle classe la donnee de
  // quelqu'un sous le mauvais patient.
  it('laisse absent un niveau que la proposition ne donne pas', () => {
    const propose = lireProposition('subject: level 0, modality: level 1');
    expect(propose).toMatchObject({ subjectLevel: 0, modalityLevel: 1 });
    expect(propose?.visitLevel).toBeUndefined();
  });

  it('refuse un texte sans niveau de regroupement', () => {
    expect(lireProposition('visit: level 2, modality: level 3')).toBeNull();
    expect(lireProposition("bonjour, je suis l'assistant")).toBeNull();
    expect(lireProposition('   ')).toBeNull();
  });

  // Le domaine refuse deja un niveau au-dela de 15 ; remplir le champ avec
  // une valeur qui sera rejetee ne rend service a personne.
  it('ignore un niveau qui est une faute de frappe', () => {
    expect(lireProposition('subject: level 1, visit: level 99')?.visitLevel).toBeUndefined();
  });
});

/* La proposition reelle de l'assistant pour SELENA, collee telle quelle.
 *
 *  Elle donne les trois positions dans la forme attendue, puis mentionne la
 *  quatrieme en prose, a l'envers : « le niveau 4 porte le type de données ».
 *  C'est ce texte-la que quelqu'un colle, et le niveau 4 n'etait pas trouve. */
const PROPOSITION_SELENA = `subject: level 1, visit: level 2, modality: level 3
names: patient / visite / modalité

Je note que le niveau 4 porte le type de données (sous-catégorie), mais que la
règle de lecture ne définit que trois positions principales : sujet, visite et
modalité. Le niveau 3 reste donc le niveau de la modalité (l'instrument ou
l'examen principal), et le niveau 4 est un sous-niveau optionnel qui ne fait
pas partie de la structure principale de lecture.`;

describe('le niveau sous la modalité', () => {
  it('se lit dans la proposition de SELENA, écrite à l’envers', () => {
    const lue = lireProposition(PROPOSITION_SELENA);
    expect(lue).not.toBeNull();
    expect(lue?.subjectLevel).toBe(1);
    expect(lue?.visitLevel).toBe(2);
    expect(lue?.modalityLevel).toBe(3);
    expect(lue?.dataTypeLevel).toBe(4);
    expect(lue?.modalityName).toBe('modalité');
  });

  it('se lit aussi dans la forme que la consigne demande', () => {
    const lue = lireProposition(
      'subject: level 1, visit: level 2, modality: level 3, data type: level 4\nnames: patient / visite / modalité / type',
    );
    expect(lue?.dataTypeLevel).toBe(4);
    expect(lue?.dataTypeName).toBe('type');
  });

  /* Une phrase qui parle d'un quatrieme niveau sans le designer ne doit rien
   *  remplir : un champ rempli de travers se voit moins qu'un champ vide. */
  it('reste absent quand la proposition n’en nomme pas', () => {
    const lue = lireProposition(
      'subject: level 1, visit: level 2, modality: level 3\nnames: patient / visite / modalité\nIl existe un sous-niveau optionnel au niveau 4 que je ne décris pas.',
    );
    expect(lue?.subjectLevel).toBe(1);
    expect(lue?.dataTypeLevel).toBeUndefined();
    expect(lue?.dataTypeName).toBeUndefined();
  });
});
