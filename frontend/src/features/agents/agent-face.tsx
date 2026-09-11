/**
 * Le visage d'un agent.
 *
 * L'anthropomorphisme n'est pas une decoration ici : il porte une information
 * qu'un badge ne porte pas. On confie une consigne permanente a quelqu'un, on
 * revient le lendemain, et la premiere question est « est-ce qu'il a fait son
 * travail ». Un visage repond a ca avant la lecture - les yeux fermes se
 * distinguent d'un regard alerte a la vitesse ou l'on parcourt une rangee.
 *
 * Donc l'expression encode l'etat et rien d'autre, et chaque etat a la sienne :
 *
 *   travaille   les yeux ouverts, le regard qui suit  (une course en cours)
 *   calme       un demi-sourire, rien a signaler
 *   trouve      les sourcils leves : il a quelque chose a dire
 *   en pause    les yeux fermes, personne ne lui a rien demande
 *   en panne    un trait soucieux, sa derniere sortie a echoue
 *
 * La teinte vient du nom, de facon deterministe : deux agents differents ne se
 * ressemblent pas, et le meme agent garde son visage d'un jour a l'autre. On
 * ne tire pas au hasard - un collegue qui change de tete a chaque rechargement
 * n'est plus un collegue.
 */

export type AgentMood = 'working' | 'calm' | 'found' | 'paused' | 'failed';

/** Teinte stable derivee du nom. Les bornes evitent le vert-jaune qui lit
 *  comme un avertissement et le rouge qui lit comme une panne : la couleur
 *  identifie, elle ne signale pas. */
function hueFor(seed: string): number {
  let hash = 0;
  for (let index = 0; index < seed.length; index += 1) {
    hash = (hash * 31 + seed.charCodeAt(index)) % 100000;
  }
  const wheel = [200, 260, 320, 20, 170, 290, 230, 340];
  return wheel[hash % wheel.length] ?? 220;
}

interface AgentFaceProps {
  name: string;
  mood: AgentMood;
  size?: number;
  className?: string;
}

export function AgentFace({ name, mood, size = 44, className }: AgentFaceProps) {
  const hue = hueFor(name);
  const skin = `hsl(${hue} 62% 88%)`;
  const skinDark = `hsl(${hue} 45% 72%)`;
  const ink = `hsl(${hue} 45% 26%)`;

  const closed = mood === 'paused';
  const brows = mood === 'found';

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 48 48"
      className={className}
      role="img"
      aria-hidden="true"
      focusable="false"
    >
      <defs>
        <linearGradient id={`face-${hue}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={skin} />
          <stop offset="100%" stopColor={skinDark} />
        </linearGradient>
      </defs>
      <rect x="1" y="1" width="46" height="46" rx="14" fill={`url(#face-${hue})`} />

      {brows ? (
        <g stroke={ink} strokeWidth="1.8" strokeLinecap="round" opacity="0.75">
          <path d="M13 16.5 q4 -2.4 8 -0.4" fill="none" />
          <path d="M27 16.1 q4 -2 8 0.4" fill="none" />
        </g>
      ) : null}

      {closed ? (
        <g stroke={ink} strokeWidth="2.4" strokeLinecap="round">
          <path d="M14 23 q3.5 2.6 7 0" fill="none" />
          <path d="M27 23 q3.5 2.6 7 0" fill="none" />
        </g>
      ) : (
        <g fill={ink}>
          <circle cx="17.5" cy="22.5" r="3.1" />
          <circle cx="30.5" cy="22.5" r="3.1" />
          {/* Le reflet : sans lui les yeux lisent comme deux trous. */}
          <circle cx="18.6" cy="21.4" r="1" fill="#fff" opacity="0.85" />
          <circle cx="31.6" cy="21.4" r="1" fill="#fff" opacity="0.85" />
        </g>
      )}

      <path
        d={mouthFor(mood)}
        fill="none"
        stroke={ink}
        strokeWidth="2.2"
        strokeLinecap="round"
        opacity="0.85"
      />
    </svg>
  );
}

function mouthFor(mood: AgentMood): string {
  switch (mood) {
    case 'found':
      // Bouche ouverte : il a quelque chose a dire.
      return 'M19 32 q5 4.5 10 0 q-5 2.2 -10 0';
    case 'failed':
      return 'M19 33.5 q5 -3.5 10 0';
    case 'paused':
      return 'M20 32.5 h8';
    default:
      return 'M19 31.5 q5 3.8 10 0';
  }
}

/** L'humeur decoule de l'etat, jamais de ce que le rapport raconte. */
export function moodOf(agent: {
  enabled: boolean;
  lastReport?: string;
  lastQuiet: boolean;
  lastRunAt?: string;
}): AgentMood {
  if (!agent.enabled) return 'paused';
  if (!agent.lastRunAt) return 'calm';
  if (agent.lastReport) return 'found';
  if (agent.lastQuiet) return 'calm';
  // Il a tourne, n'a rien rapporte et n'a pas dit « rien a signaler ».
  return 'failed';
}
