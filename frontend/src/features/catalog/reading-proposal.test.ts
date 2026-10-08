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
