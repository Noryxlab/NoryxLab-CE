import * as React from 'react';
import { createHost, type ExtensionModule } from '@/lib/extensions';
import { useI18n } from '@/lib/i18n';
import { useToast } from '@/components/ui/toast';
import { ErrorState } from './states';

/**
 * Renders one extension module into a plain DOM container.
 *
 * The extension owns the element's contents; React only owns the element
 * itself, so an extension crashing cannot corrupt the React tree around it.
 */
export function ExtensionSlot({ module }: { module: ExtensionModule }) {
  const { locale } = useI18n();
  const toast = useToast();
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [error, setError] = React.useState<unknown>(null);

  React.useEffect(() => {
    const element = containerRef.current;
    if (!element) return;

    const host = createHost(locale, (message, tone) => {
      if (tone === 'error') toast.error(message);
      else if (tone === 'success') toast.success(message);
      else toast.info(message);
    });

    let cancelled = false;
    setError(null);

    void Promise.resolve()
      .then(() => module.mount(element, host))
      .catch((cause: unknown) => {
        if (!cancelled) setError(cause);
      });

    return () => {
      cancelled = true;
      try {
        module.unmount?.(element);
      } catch (cause) {
        console.error(`Extension ${module.id} failed to unmount`, cause);
      }
      element.replaceChildren();
    };
    // The host is rebuilt when the locale changes so the extension re-renders
    // in the new language.
  }, [module, locale, toast]);

  if (error) return <ErrorState error={error} />;

  return <div ref={containerRef} data-extension={module.id} />;
}
