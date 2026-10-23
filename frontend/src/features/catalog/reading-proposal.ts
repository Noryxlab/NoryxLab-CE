/* Lire une proposition de l'assistant et la rendre applicable.
 *
 *  L'assistant repond exactement « subject: level 1, visit: level 2,
 *  modality: level 3 » puis « names: patient / visite / modalité », parce que
 *  sa consigne le lui demande - et la boucle s'arretait la. Il proposait, et
 *  il fallait recopier trois nombres et trois mots a la main depuis un
 *  panneau de discussion vers un formulaire, en se trompant de niveau une
 *  fois sur trois. Une proposition qu'on ne peut pas appliquer n'est pas une
 *  aide, c'est une dictee.
 *
 *  L'analyse se fait ici, cote Community, sur du texte colle. C'est
 *  deliberement bete : aucune dependance a l'extension qui a produit la
 *  reponse, aucun evenement a inventer entre les deux editions (ADR-015,
 *  ADR-030), et ca marche aussi quand c'est un collegue qui a ecrit la ligne
 *  dans un courriel. Le texte entier est accepte : on cherche les motifs
 *  dedans plutot que d'exiger qu'il soit seul sur sa ligne.
 *
 *  Et ca ne fait que REMPLIR le formulaire. Rien n'est enregistre, rien n'est
 *  applique : la personne voit les six champs, les corrige, essaye sur de
 *  vrais chemins, puis enregistre. C'est la regle d'ADR-040 - l'assistant
 *  propose, quelqu'un confirme - et la transcription cesse d'etre manuelle
 *  sans cesser d'etre une confirmation. */

export interface ReadingProposal {
  subjectLevel?: number;
  visitLevel?: number;
  modalityLevel?: number;
  /** Le niveau sous la modalite, quand la proposition en nomme un. Absent
   *  sinon : l'assistant decrit souvent trois positions et mentionne la
   *  quatrieme en passant, et un champ qu'il n'a pas propose ne doit pas
   *  arriver a zero dans le formulaire. */
  dataTypeLevel?: number;
  subjectName?: string;
  visitName?: string;
  modalityName?: string;
  dataTypeName?: string;
}

/** Les niveaux, dans la forme que la consigne impose a l'assistant. */
function niveau(texte: string, clef: string): number | undefined {
  // « subject: level 1 », « subject : niveau 1 », « subject level 1 ».
  const motif = new RegExp(`${clef}\\s*:?\\s*(?:level|niveau)\\s*(\\d{1,2})`, 'i');
  const trouve = motif.exec(texte);
  if (!trouve) return undefined;
  const valeur = Number(trouve[1]);
  // Au-dela de 15 ce n'est pas une mise en page, c'est une faute de frappe -
  // le domaine le refuse deja cote serveur, et le dire ici evite de remplir
  // un champ avec une valeur qui sera rejetee.
  return valeur >= 0 && valeur <= 15 ? valeur : undefined;
}

/** Le niveau sous la modalite, dans les deux sens ou une phrase le dit.
 *
 *  La consigne demande « data type: level 4 », et c'est la forme cherchee
 *  d'abord. Mais une proposition ecrite en prose le dit a l'envers - « je
 *  note que le niveau 4 porte le type de donnees (sous-categorie) » - et
 *  refuser cette phrase-la renvoie quelqu'un a recopier un chiffre a la main,
 *  ce que cette case existe precisement pour eviter.
 *
 *  Les mots acceptes sont nommes un par un plutot que devines : « sous-niveau
 *  optionnel » parle aussi du quatrieme niveau sans le designer, et une
 *  correspondance large remplirait le formulaire avec le niveau d'une phrase
 *  qui n'en proposait aucun. */
const MOTS_DU_TYPE = '(?:data\\s*type|type\\s+de\\s+donn[ée]es|sous-cat[ée]gorie|subcategory|kind)';

function niveauDuType(texte: string): number | undefined {
  const direct = niveau(texte, MOTS_DU_TYPE);
  if (direct !== undefined) return direct;
  // « le niveau 4 porte le type de donnees » : le chiffre avant les mots,
  // dans la meme proposition - pas au bout d'un paragraphe.
  const inverse = new RegExp(`(?:level|niveau)\\s*(\\d{1,2})[^.\\n]{0,40}?${MOTS_DU_TYPE}`, 'i');
  const trouve = inverse.exec(texte);
  if (!trouve) return undefined;
  const valeur = Number(trouve[1]);
  return valeur >= 0 && valeur <= 15 ? valeur : undefined;
}

/** Les mots du metier : « names: patient / visite / modalité ». */
function noms(texte: string): [string?, string?, string?, string?] {
  // Le quatrieme nom est facultatif : « patient / visite / modalité » reste
  // la forme courante, et une proposition qui ajoute « / type » la donne.
  const motif =
    /\bnames?\s*:?\s*([^\n/]{1,40})\/([^\n/]{1,40})\/([^\n/]{1,40})(?:\/([^\n/]{1,40}))?/i;
  const trouve = motif.exec(texte);
  if (!trouve) return [undefined, undefined, undefined, undefined];
  const propre = (brut: string | undefined) => {
    if (!brut) return undefined;
    const mot = brut.trim().replace(/[.;,]+$/, '');
    // Un seul mot par niveau : la consigne le demande, et une phrase entiere
    // dans une etiquette de colonne ne tient pas.
    return mot && !mot.includes(' ') ? mot : undefined;
  };
  return [propre(trouve[1]), propre(trouve[2]), propre(trouve[3]), propre(trouve[4])];
}

/** lireProposition rend ce qu'elle a compris, et rien de plus.
 *
 *  Un champ absent reste absent : un niveau que l'assistant a dit ne pas
 *  savoir ne doit pas arriver a zero dans le formulaire, parce que zero est
 *  une regle - et une regle accidentelle classe la donnee de quelqu'un sous
 *  le mauvais patient. */
export function lireProposition(texte: string): ReadingProposal | null {
  const brut = texte.trim();
  if (!brut) return null;

  const [sujet, visite, modalite, typeDonnees] = noms(brut);
  const proposition: ReadingProposal = {
    subjectLevel: niveau(brut, 'subject'),
    visitLevel: niveau(brut, 'visit'),
    modalityLevel: niveau(brut, 'modality'),
    dataTypeLevel: niveauDuType(brut),
    subjectName: sujet,
    visitName: visite,
    modalityName: modalite,
    dataTypeName: typeDonnees,
  };

  // Sans niveau de regroupement il n'y a pas de lecture : la refuser en
  // disant ce qu'on attendait vaut mieux que remplir cinq champs sur six et
  // laisser quelqu'un chercher le sixieme.
  if (proposition.subjectLevel === undefined) return null;
  return proposition;
}
