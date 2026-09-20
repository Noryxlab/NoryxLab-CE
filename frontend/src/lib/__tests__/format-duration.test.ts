import { describe, expect, it } from 'vitest';
import { formatDuration } from '../format';

// Une carte de workspace a affiche "moins d'une minute" sur un workspace de
// vingt-deux minutes le 2026-09-18. La cause etait un clamp a zero : un debut
// posterieur a la fin devenait une duree nulle, rendue comme la plus courte
// des durees. L'ecran ne disait pas qu'il ne savait pas, il donnait un chiffre
// - et ce chiffre se lit comme un workspace qui vient de demarrer.
describe('formatDuration', () => {
  const start = '2026-09-18T14:17:28Z';

  it('rend les durees normales', () => {
    expect(formatDuration(start, '2026-09-18T14:39:00Z')).toBe('21 min');
    expect(formatDuration(start, '2026-09-18T16:20:28Z')).toBe('2h 03m');
    expect(formatDuration(start, '2026-09-20T15:17:28Z')).toBe('2j 1h');
    expect(formatDuration(start, '2026-09-18T14:17:50Z')).toBe("moins d'une minute");
  });

  it("refuse d'inventer une duree quand le debut est apres la fin", () => {
    // Bien au-dela de la derive d'horloge : la donnee est fausse, et le dire
    // vaut mieux que l'arrondir a la plus petite valeur possible.
    expect(formatDuration('2026-09-18T15:00:00Z', '2026-09-18T14:00:00Z')).toBe('—');
  });

  it('tolere la derive entre les horloges du navigateur et du cluster', () => {
    // Quelques secondes d'ecart sont normales et ne doivent pas effacer une
    // duree que tout le monde attend a zero.
    expect(formatDuration('2026-09-18T14:00:30Z', '2026-09-18T14:00:00Z')).toBe(
      "moins d'une minute",
    );
  });

  it('rend un tiret sans date plutot qu une duree', () => {
    expect(formatDuration(null)).toBe('—');
    expect(formatDuration(undefined)).toBe('—');
  });
});
