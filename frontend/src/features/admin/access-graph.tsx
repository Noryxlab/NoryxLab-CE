import * as React from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { useI18n, useT } from '@/lib/i18n';
import type { RbacCell, RbacMatrixReport } from '@/lib/api/types';

/**
 * Who reaches what, drawn.
 *
 * The table answers "who has access"; an audit asks "why does this person have
 * access", and that is a question about paths. A grant inherited from an
 * organization and one given directly look identical in a list and are
 * completely different facts when somebody leaves a team.
 *
 * Bipartite and column-based rather than force-directed. A force layout of the
 * same data is prettier for one screenshot and unreadable the moment there are
 * fifty subjects: nodes drift, nothing is comparable between two visits, and
 * the eye cannot follow an edge. Two columns stay legible and stay put.
 *
 * It refuses to draw everything at once. At 23 subjects this data is readable;
 * at 200 users it is a ball of wool that answers nothing, so the view asks for
 * a focus instead of rendering something that merely looks like information.
 */

// Where a two-column diagram stops answering questions.
//
// Not tuned to make a particular platform pass: rows are cheap - the drawing
// grows taller and stays readable - while crossings are what defeat the eye,
// and past roughly this many lines you can no longer follow one from a name to
// a resource. A platform with 62 grants is well inside it; one with a thousand
// needs a filter, and should be told so rather than handed a ball of wool.
const MAX_EDGES_WITHOUT_FOCUS = 150;

const ROW_HEIGHT = 26;
const COLUMN_WIDTH = 260;
const PADDING = 16;

type Focus = { kind: 'none' } | { kind: 'subject'; id: string } | { kind: 'resource'; id: string };

function edgeTone(cell: RbacCell): string {
  if (cell.inherited) return 'var(--noryx-accent-cyan)';
  if (cell.role === 'owner') return 'var(--noryx-brand)';
  return 'var(--noryx-border-strong)';
}

export function AccessGraph({ report }: { report: RbacMatrixReport }) {
  const t = useT();
  const { locale } = useI18n();

  const [focus, setFocus] = React.useState<Focus>({ kind: 'none' });
  const [resourceType, setResourceType] = React.useState('');
  const [showInherited, setShowInherited] = React.useState(true);

  const resourceTypes = React.useMemo(
    () => [...new Set(report.resources.map((resource) => resource.type))].sort(),
    [report.resources],
  );

  const edges = React.useMemo(() => {
    return report.cells.filter((cell) => {
      if (!showInherited && cell.inherited) return false;
      if (resourceType && cell.resourceType !== resourceType) return false;
      if (focus.kind === 'subject') return cell.subjectId === focus.id;
      if (focus.kind === 'resource') return cell.resourceId === focus.id;
      return true;
    });
  }, [report.cells, focus, resourceType, showInherited]);

  // Only the nodes an edge actually touches: an isolated node on an access
  // graph says nothing and costs a row of height.
  const subjects = React.useMemo(() => {
    const seen = new Map<string, { id: string; name: string; type: string }>();
    for (const edge of edges) {
      seen.set(edge.subjectId, { id: edge.subjectId, name: edge.subjectName, type: edge.subjectType });
    }
    return [...seen.values()].sort((a, b) => a.name.localeCompare(b.name, locale));
  }, [edges, locale]);

  const resources = React.useMemo(() => {
    const seen = new Map<string, { id: string; name: string; type: string }>();
    for (const edge of edges) {
      seen.set(edge.resourceId, { id: edge.resourceId, name: edge.resourceName, type: edge.resourceType });
    }
    return [...seen.values()].sort(
      (a, b) => a.type.localeCompare(b.type) || a.name.localeCompare(b.name, locale),
    );
  }, [edges, locale]);

  const subjectRow = new Map(subjects.map((subject, index) => [subject.id, index]));
  const resourceRow = new Map(resources.map((resource, index) => [resource.id, index]));

  const height = Math.max(subjects.length, resources.length, 1) * ROW_HEIGHT + PADDING * 2;
  const width = COLUMN_WIDTH * 2 + 160;
  const tooMany = focus.kind === 'none' && edges.length > MAX_EDGES_WITHOUT_FOCUS;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end gap-2">
        <Field label={t('graph.focusSubject')} className="min-w-52 flex-1">
          <Select
            value={focus.kind === 'subject' ? focus.id : ''}
            onValueChange={(value) => setFocus(value ? { kind: 'subject', id: value } : { kind: 'none' })}
            placeholder={t('graph.everyone')}
            options={[...report.subjects]
              .sort((a, b) => a.name.localeCompare(b.name, locale))
              .map((subject) => ({ value: subject.id, label: subject.name, hint: subject.type }))}
          />
        </Field>
        <Field label={t('graph.focusResource')} className="min-w-52 flex-1">
          <Select
            value={focus.kind === 'resource' ? focus.id : ''}
            onValueChange={(value) => setFocus(value ? { kind: 'resource', id: value } : { kind: 'none' })}
            placeholder={t('graph.everything')}
            options={[...report.resources]
              .sort((a, b) => a.name.localeCompare(b.name, locale))
              .map((resource) => ({ value: resource.id, label: resource.name, hint: resource.type }))}
          />
        </Field>
        <Field label={t('graph.resourceType')} className="min-w-40">
          <Select
            value={resourceType}
            onValueChange={setResourceType}
            placeholder={t('graph.allTypes')}
            options={resourceTypes.map((type) => ({ value: type, label: type }))}
          />
        </Field>
        <Button
          variant={showInherited ? 'secondary' : 'ghost'}
          size="sm"
          onClick={() => setShowInherited((current) => !current)}
        >
          {showInherited ? t('graph.hideInherited') : t('graph.showInherited')}
        </Button>
        {focus.kind !== 'none' || resourceType ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setFocus({ kind: 'none' });
              setResourceType('');
            }}
          >
            {t('graph.clear')}
          </Button>
        ) : null}
      </div>

      <Card>
        <CardContent className="py-4">
          {edges.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">{t('graph.empty')}</p>
          ) : tooMany ? (
            // Refused rather than drawn badly: a hairball of every grant on the
            // platform looks like information and answers nothing.
            <div className="space-y-2 py-8 text-center">
              <p className="text-sm">{t('graph.tooMany', { count: String(edges.length) })}</p>
              <p className="text-xs text-muted-foreground">{t('graph.tooManyHint')}</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <svg
                viewBox={`0 0 ${width} ${height}`}
                width={width}
                height={height}
                role="img"
                aria-label={t('graph.title')}
                className="max-w-full"
              >
                {edges.map((edge, index) => {
                  const from = (subjectRow.get(edge.subjectId) ?? 0) * ROW_HEIGHT + PADDING;
                  const to = (resourceRow.get(edge.resourceId) ?? 0) * ROW_HEIGHT + PADDING;
                  const x1 = COLUMN_WIDTH;
                  const x2 = COLUMN_WIDTH + 160;
                  return (
                    <path
                      key={`${edge.subjectId}-${edge.resourceId}-${index}`}
                      d={`M ${x1} ${from} C ${x1 + 70} ${from}, ${x2 - 70} ${to}, ${x2} ${to}`}
                      fill="none"
                      stroke={edgeTone(edge)}
                      strokeWidth={edge.inherited ? 1 : 1.5}
                      // Inherited grants are dashed: the distinction is the
                      // whole point of the drawing, and it must survive being
                      // printed in black and white for an audit file.
                      strokeDasharray={edge.inherited ? '4 3' : undefined}
                      opacity={0.75}
                    >
                      <title>
                        {`${edge.subjectName} → ${edge.resourceName} (${edge.role}, ${edge.source})`}
                      </title>
                    </path>
                  );
                })}

                {subjects.map((subject, index) => (
                  <text
                    key={subject.id}
                    x={COLUMN_WIDTH - 8}
                    y={index * ROW_HEIGHT + PADDING + 4}
                    textAnchor="end"
                    className="fill-foreground"
                    fontSize="12"
                  >
                    {subject.name}
                    <tspan className="fill-muted-foreground" fontSize="10">{`  ${subject.type}`}</tspan>
                  </text>
                ))}

                {resources.map((resource, index) => (
                  <text
                    key={resource.id}
                    x={COLUMN_WIDTH + 168}
                    y={index * ROW_HEIGHT + PADDING + 4}
                    className="fill-foreground"
                    fontSize="12"
                  >
                    {resource.name}
                    <tspan className="fill-muted-foreground" fontSize="10">{`  ${resource.type}`}</tspan>
                  </text>
                ))}
              </svg>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
        <span className="flex items-center gap-1.5">
          <svg width="26" height="6" aria-hidden>
            <line x1="0" y1="3" x2="26" y2="3" stroke="var(--noryx-border-strong)" strokeWidth="1.5" />
          </svg>
          {t('graph.legendDirect')}
        </span>
        <span className="flex items-center gap-1.5">
          <svg width="26" height="6" aria-hidden>
            <line x1="0" y1="3" x2="26" y2="3" stroke="var(--noryx-accent-cyan)" strokeWidth="1" strokeDasharray="4 3" />
          </svg>
          {t('graph.legendInherited')}
        </span>
        <span className="flex items-center gap-1.5">
          <svg width="26" height="6" aria-hidden>
            <line x1="0" y1="3" x2="26" y2="3" stroke="var(--noryx-brand)" strokeWidth="1.5" />
          </svg>
          {t('graph.legendOwner')}
        </span>
        <Badge tone="outline">{t('graph.edges', { count: String(edges.length) })}</Badge>
      </div>
    </div>
  );
}
