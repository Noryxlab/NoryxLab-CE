import { config } from './config';

/**
 * Asking for the assistant without knowing whether it exists.
 *
 * The assistant is an Enterprise extension and this screen is Community, so
 * Community cannot call it, import it, or hold a type from it (ADR-015,
 * ADR-030). What it can do is say out loud that somebody asked, and let
 * whatever is listening decide.
 *
 * An event rather than a callback for the same reason the extension contract
 * passes a bare element: the two sides must not share a framework, a version
 * or a build. Community carries one string.
 */
export const ASSISTANT_OPEN_EVENT = 'noryx:assistant:open';

export interface AssistantOpenRequest {
  /** Which conversation this is: the assistant carries one instruction per
   *  surface, and the ontology one proposes a reading of path shapes without
   *  being able to apply anything (ADR-040). */
  surface: 'platform' | 'ontology';
  /** What the screen already knows, handed over so the person does not have to
   *  retype it. Never anything identifying: on a regulated dataset a real key
   *  is a patient identifier, and the ontology surface is shown shapes. */
  context?: Record<string, unknown>;
  /** A first question, when the screen knows what is being asked. */
  prompt?: string;
}

export function requestAssistant(request: AssistantOpenRequest): void {
  window.dispatchEvent(new CustomEvent(ASSISTANT_OPEN_EVENT, { detail: request }));
}

/**
 * Whether anything is listening.
 *
 * A button that dispatches into an empty room is worse than no button: it
 * looks broken rather than absent. Community asks the configuration, which
 * already lists the extensions this installation declares, instead of probing
 * for code it must not know about.
 */
export function assistantAvailable(): boolean {
  return (config.extensions ?? []).some((extension) =>
    extension.id.toLowerCase().includes('assistant'),
  );
}
