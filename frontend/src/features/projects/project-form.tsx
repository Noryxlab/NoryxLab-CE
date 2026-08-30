import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input, Textarea } from '@/components/ui/input';
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
import { useT } from '@/lib/i18n';
import { projectsApi } from '@/lib/api/endpoints';
import { qk, useInvalidate } from '@/lib/api/queries';

/**
 * Project creation.
 *
 * Replaces the `<details class="action-drawer">` accordion that hid the create
 * form behind a disclosure triangle with two unlabelled inputs
 * (`placeholder="Nom du nouveau projet"`). Labels are real, the primary action
 * is disabled until the form is valid, and the user lands inside the project
 * that was just created rather than back on a list.
 */
export function CreateProjectSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const navigate = useNavigate();
  const invalidate = useInvalidate();

  const [name, setName] = React.useState('');
  const [description, setDescription] = React.useState('');
  const [touched, setTouched] = React.useState(false);

  React.useEffect(() => {
    if (!open) {
      setName('');
      setDescription('');
      setTouched(false);
    }
  }, [open]);

  const trimmed = name.trim();
  const nameError = touched && !trimmed ? t('common.required') : undefined;

  const mutation = useMutation({
    mutationFn: () => projectsApi.create({ name: trimmed, description: description.trim() }),
    onSuccess: (project) => {
      invalidate(qk.projects);
      onOpenChange(false);
      toast.success(project.name, t('projects.create'));
      navigate(`/projects/${project.id}`);
    },
    onError: (error) => toast.error(error, t('projects.createTitle')),
  });

  function submit(event: React.FormEvent) {
    event.preventDefault();
    setTouched(true);
    if (!trimmed) return;
    mutation.mutate();
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent aria-describedby={undefined}>
        <form onSubmit={submit} className="flex min-h-0 flex-1 flex-col">
          <SheetHeader>
            <SheetTitle>{t('projects.createTitle')}</SheetTitle>
            <SheetDescription>{t('projects.createHint')}</SheetDescription>
          </SheetHeader>
          <SheetBody>
            <Field
              label={t('projects.nameLabel')}
              description={t('projects.nameHint')}
              error={nameError}
              required
            >
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                onBlur={() => setTouched(true)}
                placeholder={t('projects.namePlaceholder')}
                autoFocus
                maxLength={120}
              />
            </Field>
            <Field label={t('projects.descriptionLabel')} description={t('projects.descriptionHint')}>
              <Textarea
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                rows={3}
                maxLength={500}
              />
            </Field>
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" variant="primary" loading={mutation.isPending} disabled={!trimmed}>
              {t('common.create')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}
