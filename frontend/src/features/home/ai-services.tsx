import { Sparkles } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useAIServices } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';

/**
 * L'etat des services d'IA, en un coup d'oeil.
 *
 * L'assistant, l'assistant de code et les agents repondent tous par la meme
 * passerelle de modeles : ils partagent donc un seul etat, et une personne qui
 * ouvre la plateforme doit le voir avant de le decouvrir d'un assistant qui ne
 * repond pas.
 *
 * Les trois etats disent des choses differentes. Complet, tout fonctionne.
 * Reduit veut dire qu'un petit modele prend le relais : les questions simples
 * passent, l'assistance au code et les agents non - et le dire vaut mieux que
 * de laisser un petit modele repondre a une question difficile avec aplomb et
 * a cote. Arrete veut dire que rien ne sert.
 */
export function AIServicesCard() {
  const t = useT();
  const status = useAIServices();

  // Rien du tout la ou aucune passerelle n'est installee : un voyant rouge pour
  // une brique absente est une fausse alerte, pas une information.
  if (!status.data?.configured) return null;

  const mode = status.data.mode ?? 'down';
  const tone = ({ full: 'success', degraded: 'warning', down: 'danger' } as const)[mode];
  const label = t(`aiServices.${mode}`);

  return (
    <Card>
      <CardContent className="flex items-start gap-3 py-4">
        <Sparkles className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium">{t('aiServices.title')}</span>
            <Badge tone={tone}>{label}</Badge>
          </div>
          {/* Le detail n'apparait que lorsqu'il y a quelque chose a comprendre :
              un service complet n'a rien a expliquer. */}
          {status.data.detail ? (
            <p className="text-xs leading-relaxed text-muted-foreground">{status.data.detail}</p>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}
