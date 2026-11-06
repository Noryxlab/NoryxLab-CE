import * as React from 'react';
import { Field } from '@/components/ui/field';
import { useT } from '@/lib/i18n';
import { defaultWindow, paintSlice, readDicom, type DicomSlice } from './dicom';

/* Une coupe a l'ecran, avec la seule commande qui compte.
 *
 * Le fenetrage n'est pas un reglage de confort : un scanner couvre des
 * milliers de niveaux, un ecran en montre 256, et une image affichee hors de
 * sa fenetre est grise uniforme. C'est la difference entre « le fichier est
 * vide » et « le fichier est la ».
 *
 * Deliberement pas un lecteur de diagnostic : pas de mesure, pas de
 * superposition, pas de navigation dans une serie. Relire une etude se fait
 * dans Slicer, qui est un bureau distant sans canal de fichier. */
export function DicomView({ bytes }: { bytes: ArrayBuffer }) {
  const t = useT();
  const canvas = React.useRef<HTMLCanvasElement | null>(null);
  const read = React.useMemo(() => readDicom(bytes), [bytes]);
  const slice = read.kind === 'image' ? (read as DicomSlice) : null;

  const [window, setWindow] = React.useState<{ centre: number; width: number } | null>(null);
  React.useEffect(() => {
    setWindow(slice ? defaultWindow(slice) : null);
  }, [slice]);

  React.useEffect(() => {
    const element = canvas.current;
    if (!element || !slice || !window) return;
    const context = element.getContext('2d');
    if (!context) return;
    const picture = context.createImageData(slice.columns, slice.rows);
    paintSlice(slice, window.centre, window.width, picture);
    context.putImageData(picture, 0, 0);
  }, [slice, window]);

  if (read.kind === 'refused') {
    return (
      <p className="text-sm text-muted-foreground">
        {read.reason === 'compressed'
          ? t('datasets.dicomCompressed', { syntax: read.syntax ?? '' })
          : read.reason === 'unsupported'
            ? t('datasets.dicomUnsupported')
            : t('datasets.dicomMalformed')}
      </p>
    );
  }
  if (!slice || !window) return null;

  const span = Math.max(slice.maximum - slice.minimum, 1);

  return (
    <div className="space-y-3">
      <canvas
        ref={canvas}
        width={slice.columns}
        height={slice.rows}
        /* Les pixels sont les pixels : un lissage ferait inventer au
           navigateur des niveaux qui ne sont pas dans le fichier. */
        className="mx-auto max-h-[55vh] w-auto max-w-full rounded-md bg-black [image-rendering:pixelated]"
      />
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label={t('datasets.dicomWindowCentre')}>
          <input
            type="range"
            min={slice.minimum - span}
            max={slice.maximum + span}
            step={Math.max(span / 500, 0.01)}
            value={window.centre}
            onChange={(event) => setWindow({ ...window, centre: Number(event.target.value) })}
            className="w-full"
          />
        </Field>
        <Field label={t('datasets.dicomWindowWidth')}>
          <input
            type="range"
            min={1}
            max={span * 3}
            step={Math.max(span / 500, 0.01)}
            value={window.width}
            onChange={(event) => setWindow({ ...window, width: Number(event.target.value) })}
            className="w-full"
          />
        </Field>
      </div>
      <p className="text-xs text-muted-foreground">
        {t('datasets.dicomSummary', {
          modality: slice.modality || '—',
          size: `${slice.columns}×${slice.rows}`,
          centre: Math.round(window.centre).toString(),
          width: Math.round(window.width).toString(),
        })}
      </p>
    </div>
  );
}
