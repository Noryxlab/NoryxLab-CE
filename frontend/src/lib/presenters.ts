import type { HardwareTier } from './api/types';
import type { Locale } from './i18n';

/**
 * Presenters translate backend vocabulary into product vocabulary.
 *
 * The API contract is unchanged — tier ids stay `1x4`, commands stay shell
 * strings — but none of that reaches the screen raw. This is where the
 * previous UI leaked its implementation at the user:
 *
 *   "Les profils sont nommés CPUxRAM : 1x4 signifie 1 vCPU et 4 Gi RAM."
 *   <input id="dashboardCommand"
 *          value="python3 -m http.server 9000 --bind 0.0.0.0 --directory /mnt" />
 *
 * If a naming scheme needs a help note to be understood, the naming scheme is
 * the problem, so tiers get size names and apps get frameworks.
 */

/* -- hardware tiers -------------------------------------------------------- */

const TIER_NAMES: Record<string, { fr: string; en: string }> = {
  '0.5x2': { fr: 'Découverte', en: 'Discovery' },
  '1x4': { fr: 'Standard', en: 'Standard' },
  '2x8': { fr: 'Confortable', en: 'Large' },
  '4x16': { fr: 'Intensif', en: 'Extra large' },
};

/** Parses a Kubernetes CPU limit ("500m", "2") into cores. */
function tierCores(cpuLimit: string): number | null {
  const trimmed = cpuLimit.trim();
  if (!trimmed) return null;
  if (trimmed.endsWith('m')) {
    const value = Number(trimmed.slice(0, -1));
    return Number.isFinite(value) ? value / 1000 : null;
  }
  const value = Number(trimmed);
  return Number.isFinite(value) ? value : null;
}

/** Parses a Kubernetes memory limit ("4Gi", "512Mi") into gibibytes. */
function tierMemoryGiB(memoryLimit: string): number | null {
  const match = /^(\d+(?:\.\d+)?)(Gi|Mi|G|M)$/.exec(memoryLimit.trim());
  if (!match) return null;
  const amount = Number(match[1]);
  switch (match[2]) {
    case 'Gi':
    case 'G':
      return amount;
    case 'Mi':
    case 'M':
      return amount / 1024;
    default:
      return null;
  }
}

export interface TierPresentation {
  /** Human name shown as the primary label. */
  name: string;
  /** Resource summary shown underneath, e.g. "1 vCPU · 4 Go de RAM". */
  specs: string;
}

export function presentTier(tier: HardwareTier, locale: Locale = 'fr'): TierPresentation {
  const named = TIER_NAMES[tier.id];
  const cores = tierCores(tier.cpuLimit);
  const memory = tierMemoryGiB(tier.memoryLimit);

  const coreLabel =
    cores === null ? null : `${new Intl.NumberFormat(locale === 'fr' ? 'fr-FR' : 'en-US').format(cores)} vCPU`;
  const memoryLabel =
    memory === null
      ? null
      : locale === 'fr'
        ? `${new Intl.NumberFormat('fr-FR').format(memory)} Go de RAM`
        : `${new Intl.NumberFormat('en-US').format(memory)} GB RAM`;

  const specs = [coreLabel, memoryLabel].filter(Boolean).join(' · ');

  return {
    name: named ? named[locale] : tier.name,
    // Fall back to whatever the backend described if the limits are unparseable,
    // so an unknown custom tier still shows something meaningful.
    specs: specs || tier.description || tier.id,
  };
}

/* -- workspace storage ----------------------------------------------------- */

/** Preset sizes, replacing the free-text "Stockage (ex. 10Gi)" input. */
export const STORAGE_PRESETS = [
  { value: '10Gi', gib: 10 },
  { value: '25Gi', gib: 25 },
  { value: '50Gi', gib: 50 },
  { value: '100Gi', gib: 100 },
  { value: '250Gi', gib: 250 },
] as const;

export function presentStorage(gib: number, locale: Locale = 'fr'): string {
  const formatted = new Intl.NumberFormat(locale === 'fr' ? 'fr-FR' : 'en-US').format(gib);
  return locale === 'fr' ? `${formatted} Go` : `${formatted} GB`;
}

/* -- IDE ------------------------------------------------------------------- */

const IDE_LABELS: Record<string, string> = {
  jupyter: 'JupyterLab',
  vscode: 'VS Code',
  rstudio: 'RStudio',
};

export function presentIde(ide: string | null | undefined): string {
  if (!ide) return '—';
  return IDE_LABELS[ide.toLowerCase()] ?? ide;
}

export function presentIdes(ides: string[] | null | undefined): string {
  if (!ides || ides.length === 0) return '—';
  return ides.map(presentIde).join(', ');
}

/* -- application frameworks ------------------------------------------------ */

export interface AppFramework {
  id: string;
  label: { fr: string; en: string };
  /** Short description shown as a hint in the framework picker. */
  hint: { fr: string; en: string };
  defaultPort: number;
  defaultEntrypoint: string;
  /** Builds the launch command. `entrypoint` is relative to the project root. */
  command: (entrypoint: string, port: number) => string[];
}

export const APP_FRAMEWORKS: AppFramework[] = [
  {
    id: 'streamlit',
    label: { fr: 'Streamlit', en: 'Streamlit' },
    hint: {
      fr: 'Application Python interactive, la plus courante pour partager une analyse.',
      en: 'Interactive Python app, the most common way to share an analysis.',
    },
    defaultPort: 8501,
    defaultEntrypoint: 'app.py',
    command: (entrypoint, port) => [
      'streamlit',
      'run',
      entrypoint,
      '--server.port',
      String(port),
      '--server.address',
      '0.0.0.0',
      '--server.headless',
      'true',
    ],
  },
  {
    id: 'dash',
    label: { fr: 'Dash', en: 'Dash' },
    hint: {
      fr: 'Application Python orientée tableaux de bord analytiques.',
      en: 'Python app oriented towards analytical dashboards.',
    },
    defaultPort: 8050,
    defaultEntrypoint: 'app.py',
    command: (entrypoint) => ['python', entrypoint],
  },
  {
    id: 'shiny',
    label: { fr: 'Shiny (R)', en: 'Shiny (R)' },
    hint: {
      fr: 'Application R interactive.',
      en: 'Interactive R application.',
    },
    defaultPort: 3838,
    defaultEntrypoint: 'app.R',
    command: (_entrypoint, port) => [
      'R',
      '-e',
      `shiny::runApp('.', host='0.0.0.0', port=${port})`,
    ],
  },
  {
    id: 'gradio',
    label: { fr: 'Gradio', en: 'Gradio' },
    hint: {
      fr: 'Interface web pour démonstrations de modèles.',
      en: 'Web interface for model demonstrations.',
    },
    defaultPort: 7860,
    defaultEntrypoint: 'app.py',
    command: (entrypoint) => ['python', entrypoint],
  },
  {
    id: 'static',
    label: { fr: 'Site statique', en: 'Static site' },
    hint: {
      fr: 'Sert un dossier de fichiers HTML déjà générés.',
      en: 'Serves a folder of already generated HTML files.',
    },
    defaultPort: 9000,
    defaultEntrypoint: 'public',
    command: (entrypoint, port) => [
      'python3',
      '-m',
      'http.server',
      String(port),
      '--bind',
      '0.0.0.0',
      '--directory',
      entrypoint.startsWith('/') ? entrypoint : `/mnt/${entrypoint}`,
    ],
  },
  {
    id: 'custom',
    label: { fr: 'Commande personnalisée', en: 'Custom command' },
    hint: {
      fr: 'Vous fournissez vous-même la commande de lancement.',
      en: 'You provide the launch command yourself.',
    },
    defaultPort: 8080,
    defaultEntrypoint: '',
    command: () => [],
  },
];

export function findFramework(id: string): AppFramework | undefined {
  return APP_FRAMEWORKS.find((framework) => framework.id === id);
}

export function frameworkOptions(locale: Locale = 'fr') {
  return APP_FRAMEWORKS.map((framework) => ({
    value: framework.id,
    label: framework.label[locale],
    hint: framework.hint[locale],
  }));
}

/** Shell-quotes a command array for display in a read-only preview. */
export function formatCommand(command: string[]): string {
  return command
    .map((part) => (/[\s'"]/.test(part) ? `'${part.replace(/'/g, "'\\''")}'` : part))
    .join(' ');
}

/* -- cron ------------------------------------------------------------------ */

/** Describes a 5-field cron expression in plain language, falling back to the
 *  raw expression when it does not match a common shape. */
export function describeCron(expression: string, locale: Locale = 'fr'): string {
  const parts = expression.trim().split(/\s+/);
  if (parts.length !== 5) return expression;
  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts as [string, string, string, string, string];

  const isNumber = (value: string) => /^\d+$/.test(value);
  const time =
    isNumber(minute) && isNumber(hour)
      ? `${hour.padStart(2, '0')}:${minute.padStart(2, '0')}`
      : null;

  if (time && dayOfMonth === '*' && month === '*') {
    if (dayOfWeek === '*') {
      return locale === 'fr' ? `tous les jours à ${time}` : `every day at ${time}`;
    }
    if (dayOfWeek === '1-5') {
      return locale === 'fr' ? `du lundi au vendredi à ${time}` : `Monday to Friday at ${time}`;
    }
  }

  if (minute === '0' && hour === '*' && dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    return locale === 'fr' ? 'toutes les heures' : 'every hour';
  }

  return expression;
}

export const CRON_PRESETS = [
  { value: '0 8 * * 1-5', fr: 'Jours ouvrés à 08:00', en: 'Weekdays at 08:00' },
  { value: '0 6 * * *', fr: 'Tous les jours à 06:00', en: 'Every day at 06:00' },
  { value: '0 * * * *', fr: 'Toutes les heures', en: 'Every hour' },
  { value: '0 3 * * 1', fr: 'Chaque lundi à 03:00', en: 'Every Monday at 03:00' },
] as const;

/* -- roles and access ------------------------------------------------------ */

export function presentRole(role: string | null | undefined, locale: Locale = 'fr'): string {
  const value = String(role ?? '').toLowerCase();
  const labels: Record<string, { fr: string; en: string }> = {
    reader: { fr: 'Lecteur', en: 'Reader' },
    viewer: { fr: 'Lecteur', en: 'Viewer' },
    writer: { fr: 'Contributeur', en: 'Writer' },
    editor: { fr: 'Contributeur', en: 'Editor' },
    admin: { fr: 'Administrateur', en: 'Admin' },
    owner: { fr: 'Propriétaire', en: 'Owner' },
  };
  return labels[value]?.[locale] ?? role ?? '—';
}

export function presentAccessMode(mode: string | null | undefined, locale: Locale = 'fr'): string {
  const value = String(mode ?? '').toLowerCase();
  const labels: Record<string, { fr: string; en: string }> = {
    private: { fr: 'Membres du projet', en: 'Project members' },
    project: { fr: 'Membres du projet', en: 'Project members' },
    organization: { fr: 'Organisation', en: 'Organisation' },
    authenticated: { fr: 'Utilisateurs authentifiés', en: 'Authenticated users' },
    public: { fr: 'Utilisateurs authentifiés', en: 'Authenticated users' },
  };
  return labels[value]?.[locale] ?? mode ?? '—';
}
