import * as React from 'react';
import { api, downloadFile } from './api/client';
import { config, type ExtensionDescriptor } from './config';
import { formatBytes, formatDateTime, formatDuration, formatRelative } from './format';

/**
 * Frontend extension points (ADR-015).
 *
 * The Enterprise Edition previously added UI by string-substituting into the
 * Community index.html at exact source markers:
 *
 *   html_marker = '<article class="card span-12" data-admin-section="audit">\n<h3>Audit</h3>'
 *   js_marker   = "if (authMode === 'oidc') {"
 *
 * Any refactor of CE silently broke EE, and the injected code depended on CE's
 * internal globals (escapeHTML, callJSON, badge). This replaces that with a
 * declared contract: EE ships a self-contained ES module, CE mounts it at a
 * named point and passes it a stable host object.
 *
 * The contract is deliberately framework-agnostic — mount receives a plain
 * DOM element — so an extension never has to match CE's React version.
 */

export type { ExtensionDescriptor };

export type ExtensionMountPoint =
  | 'admin.section'
  | 'project.section'
  | 'catalog.section'
  // Floats above the shell on every screen. This is what an assistant
  // launcher needs, and the reason it is a mount point rather than a CE
  // feature: Community must not carry the code it cannot run.
  | 'shell.overlay';

export interface ExtensionHost {
  /** Authenticated API access, identical to what CE screens use. */
  api: typeof api;
  downloadFile: typeof downloadFile;
  format: {
    bytes: typeof formatBytes;
    dateTime: typeof formatDateTime;
    duration: typeof formatDuration;
    relative: typeof formatRelative;
  };
  locale: 'fr' | 'en';
  edition: 'ce' | 'ee';
  /** Escapes text for safe insertion into innerHTML. */
  escapeHTML: (value: unknown) => string;
  notify: (message: string, tone?: 'info' | 'success' | 'warning' | 'error') => void;
}

export interface ExtensionModule {
  id: string;
  mountPoint: ExtensionMountPoint;
  /** Tab label, per locale. */
  title: { fr: string; en: string };
  mount: (element: HTMLElement, host: ExtensionHost) => void | Promise<void>;
  unmount?: (element: HTMLElement) => void;
}

export function escapeHTML(value: unknown): string {
  return String(value ?? '').replace(/[&<>"']/g, (character) => {
    switch (character) {
      case '&':
        return '&amp;';
      case '<':
        return '&lt;';
      case '>':
        return '&gt;';
      case '"':
        return '&quot;';
      default:
        return '&#39;';
    }
  });
}

// Declared extensions come from the runtime configuration, already validated
// and narrowed to same-origin URLs by config.ts.
function declaredExtensions(): ExtensionDescriptor[] {
  return config.extensions;
}

let loaded: Promise<ExtensionModule[]> | null = null;

async function loadAll(): Promise<ExtensionModule[]> {
  const descriptors = declaredExtensions();
  const modules = await Promise.all(
    descriptors.map(async (descriptor) => {
      try {
        const imported = (await import(/* @vite-ignore */ descriptor.url)) as {
          default?: ExtensionModule;
        };
        const module = imported.default;
        if (!module || typeof module.mount !== 'function') {
          console.warn(`Extension ${descriptor.id} has no valid default export`);
          return null;
        }
        return { ...module, id: module.id || descriptor.id };
      } catch (error) {
        // A broken extension must never take the platform down with it —
        // that is the failure mode the 2026-05-04 incident demonstrated.
        console.error(`Extension ${descriptor.id} failed to load`, error);
        return null;
      }
    }),
  );
  return modules.filter((module): module is ExtensionModule => module !== null);
}

export function useExtensions(mountPoint: ExtensionMountPoint): ExtensionModule[] {
  const [modules, setModules] = React.useState<ExtensionModule[]>([]);

  React.useEffect(() => {
    let cancelled = false;
    loaded ??= loadAll();
    void loaded.then((all) => {
      if (!cancelled) setModules(all.filter((module) => module.mountPoint === mountPoint));
    });
    return () => {
      cancelled = true;
    };
  }, [mountPoint]);

  return modules;
}

export function createHost(
  locale: 'fr' | 'en',
  notify: ExtensionHost['notify'],
): ExtensionHost {
  return {
    api,
    downloadFile,
    format: {
      bytes: formatBytes,
      dateTime: formatDateTime,
      duration: formatDuration,
      relative: formatRelative,
    },
    locale,
    edition: config.edition,
    escapeHTML,
    notify,
  };
}
