import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Plus, Trash2, Variable } from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { useToast } from '@/components/ui/toast';
import { projectVariablesApi } from '@/lib/api/endpoints';
import { qk, useInvalidate, useProjectVariables } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import type { ProjectVariable } from '@/lib/api/types';

/**
 * A project's environment variables.
 *
 * The distinction this screen exists to make: a secret belongs to a person, a
 * variable belongs to the work. Everybody on the project gets the same value,
 * injected under its own name - MLFLOW_TRACKING_URI is what mlflow looks for.
 *
 * A viewer sees the names and not the values, because a name says what a
 * workload expects and a value does not need saying. They cannot launch
 * either, so the value would never reach their shell.
 */
export function ProjectVariablesSection({ projectId }: { projectId: string }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();
  const variables = useProjectVariables(projectId);

  const [open, setOpen] = React.useState(false);
  const [name, setName] = React.useState('');
  const [value, setValue] = React.useState('');
  const [description, setDescription] = React.useState('');

  const canWrite = variables.data?.canReadValues ?? false;
  const items = variables.data?.items ?? [];

  const save = useMutation({
    mutationFn: () => projectVariablesApi.set(projectId, name.trim(), value, description.trim()),
    onSuccess: () => {
      invalidate(qk.projectVariables(projectId));
      setOpen(false);
      setName('');
      setValue('');
      setDescription('');
      toast.success(t('projectVariables.saved'), t('projectVariables.title'));
    },
    onError: (error) => toast.error(error, t('projectVariables.title')),
  });

  const remove = useMutation({
    mutationFn: (variable: ProjectVariable) => projectVariablesApi.remove(projectId, variable.name),
    onSuccess: () => {
      invalidate(qk.projectVariables(projectId));
      toast.success(t('projectVariables.removed'), t('projectVariables.title'));
    },
    onError: (error) => toast.error(error, t('projectVariables.title')),
  });

  const columns: Column<ProjectVariable>[] = [
    {
      id: 'name',
      header: t('projectVariables.nameColumn'),
      cell: (item) => <span className="font-mono text-xs">{item.name}</span>,
    },
    {
      id: 'value',
      header: t('projectVariables.valueColumn'),
      cell: (item) =>
        canWrite ? (
          <span className="font-mono text-xs text-muted-foreground">{item.value || '—'}</span>
        ) : (
          <span className="text-xs text-muted-foreground">{t('projectVariables.hidden')}</span>
        ),
    },
    {
      id: 'description',
      header: t('projectVariables.descriptionColumn'),
      cell: (item) => <span className="text-xs text-muted-foreground">{item.description || '—'}</span>,
    },
    {
      id: 'updated',
      header: t('projectVariables.updatedColumn'),
      cell: (item) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(item.updatedAt)}
          {item.updatedBy ? ` · ${item.updatedBy}` : ''}
        </span>
      ),
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('projectVariables.title')}</CardTitle>
          <CardDescription>{t('projectVariables.hint')}</CardDescription>
        </CardHeaderText>
        {canWrite ? (
          <Button variant="secondary" size="sm" onClick={() => setOpen(true)}>
            <Plus aria-hidden />
            {t('projectVariables.add')}
          </Button>
        ) : null}
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <EmptyState
            icon={Variable}
            title={t('projectVariables.emptyTitle')}
            description={t('projectVariables.emptyHint')}
          />
        ) : (
          <DataTable
            data={items}
            columns={columns}
            rowKey={(item) => item.name}
            isLoading={variables.isLoading}
            isError={variables.isError}
            error={variables.error}
            onRetry={() => void variables.refetch()}
            defaultSort={{ columnId: 'name', direction: 'asc' }}
            rowActions={
              canWrite
                ? (item) => (
                    <>
                      <DropdownMenuItem
                        onSelect={() => {
                          setName(item.name);
                          setValue(item.value ?? '');
                          setDescription(item.description ?? '');
                          setOpen(true);
                        }}
                      >
                        {t('common.edit')}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        destructive
                        onSelect={() =>
                          ask({
                            title: t('projectVariables.removeTitle'),
                            description: t('projectVariables.removeHint', { name: item.name }),
                            confirmLabel: t('common.delete'),
                            destructive: true,
                            onConfirm: () => remove.mutateAsync(item),
                          })
                        }
                      >
                        <Trash2 aria-hidden />
                        {t('common.delete')}
                      </DropdownMenuItem>
                    </>
                  )
                : undefined
            }
          />
        )}
      </CardContent>

      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{t('projectVariables.add')}</SheetTitle>
            <SheetDescription>{t('projectVariables.formHint')}</SheetDescription>
          </SheetHeader>
          <SheetBody className="space-y-4">
            <Field label={t('projectVariables.nameColumn')} description={t('projectVariables.nameHint')} required>
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="MLFLOW_TRACKING_URI"
                maxLength={64}
                className="font-mono"
              />
            </Field>
            <Field label={t('projectVariables.valueColumn')} required>
              <Input
                value={value}
                onChange={(event) => setValue(event.target.value)}
                maxLength={8192}
                className="font-mono"
              />
            </Field>
            <Field
              label={t('projectVariables.descriptionColumn')}
              description={t('projectVariables.descriptionHint')}
            >
              <Input value={description} onChange={(event) => setDescription(event.target.value)} maxLength={200} />
            </Field>
          </SheetBody>
          <SheetFooter>
            <Button variant="secondary" onClick={() => setOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              loading={save.isPending}
              disabled={!name.trim()}
              onClick={() => save.mutate()}
            >
              {t('common.save')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
      {dialog}
    </Card>
  );
}
