package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	flowdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/flow"
)

// L'ordonnanceur des flows.
//
// Un balayage, pas une goroutine par run : l'etat est en base, donc une
// livraison au milieu d'un flow de six heures le reprend ou il en etait au
// lieu de le perdre. C'est la regle que le paquet workflow a tiree de
// LangGraph, et elle vaut ici pour une raison plus brutale encore - un flow
// perdu laisse des pods en vol que plus rien ne reconcilie.
//
// Chaque passage fait trois choses, dans cet ordre, et l'ordre compte :
// relever les etapes terminees, propager les echecs, puis lancer ce qui peut
// partir. Lancer avant de relever ferait demarrer une etape dont la
// dependance vient d'echouer.
const flowSchedulerInterval = 20 * time.Second

func (h Handlers) StartFlowScheduler(ctx context.Context) {
	if h.flowStore == nil || h.jobStore == nil {
		return
	}
	log.Printf("flow scheduler started (every %s)", flowSchedulerInterval)
	go func() {
		ticker := time.NewTicker(flowSchedulerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.sweepFlows(ctx)
			}
		}
	}()
}

func (h Handlers) sweepFlows(ctx context.Context) {
	runs, err := h.flowStore.ListActiveRuns()
	if err != nil {
		log.Printf("flow scheduler: cannot list runs: %v", err)
		return
	}
	for _, run := range runs {
		h.advanceFlowRun(ctx, run)
	}
}

func (h Handlers) advanceFlowRun(ctx context.Context, run flowdomain.Run) {
	definition, found, err := h.flowStore.GetFlow(run.FlowID)
	if err != nil || !found {
		return
	}
	steps, err := h.flowStore.ListStepRuns(run.ID)
	if err != nil {
		return
	}

	states := map[string]string{}
	byKey := map[string]flowdomain.StepRun{}
	for _, step := range steps {
		states[step.StepKey] = step.Status
		byKey[step.StepKey] = step
	}

	// 1. Ce qui tournait et ne tourne plus.
	running := 0
	for _, step := range steps {
		if step.Status != flowdomain.StepRunning {
			continue
		}
		status, known := h.jobOutcome(step.JobID)
		if !known {
			running++
			continue
		}
		finished := time.Now().UTC()
		updated := step
		updated.Status = status
		updated.FinishedAt = &finished
		if err := h.flowStore.UpdateStepRun(updated); err == nil {
			states[step.StepKey] = status
			byKey[step.StepKey] = updated
		} else {
			running++
		}
	}

	// 2. Ce qui ne pourra plus tourner.
	for key, reason := range flowdomain.Propagate(definition.Steps, states) {
		step := byKey[key]
		step.RunID, step.StepKey = run.ID, key
		step.Status, step.Reason = flowdomain.StepSkipped, reason
		_ = h.flowStore.UpdateStepRun(step)
	}

	// 3. Ce qui peut partir.
	for _, key := range flowdomain.Runnable(definition.Steps, states, definition.MaxParallel, running) {
		step := byKey[key]
		step.RunID, step.StepKey = run.ID, key
		if jobID, err := h.launchFlowStep(ctx, run, definition, key); err != nil {
			started := time.Now().UTC()
			step.Status = flowdomain.StepFailed
			step.StartedAt, step.FinishedAt = &started, &started
			// La raison est gardee telle quelle : « image introuvable » et
			// « palier inconnu » demandent deux corrections differentes, et
			// un « echec de lancement » generique les confondrait.
			step.Reason = err.Error()
			states[key] = flowdomain.StepFailed
		} else {
			started := time.Now().UTC()
			step.Status, step.JobID, step.StartedAt = flowdomain.StepRunning, jobID, &started
			states[key] = flowdomain.StepRunning
			running++
		}
		_ = h.flowStore.UpdateStepRun(step)
	}

	// 4. Le run est-il termine ?
	if settled, status := flowdomain.Settled(definition.Steps, states); settled {
		if err := h.flowStore.UpdateRunStatus(run.ID, status); err == nil {
			log.Printf("flow %s run %s finished: %s", definition.Name, run.ID, status)
		}
	}
}

// jobOutcome traduit l'etat d'un job en etat d'etape, et dit s'il est tranche.
//
// Un job disparu compte comme un echec : la seule autre lecture possible
// serait d'attendre indefiniment une reponse qui ne viendra pas, et un flow
// bloque pour toujours est pire qu'un flow en echec.
func (h Handlers) jobOutcome(jobID string) (string, bool) {
	if strings.TrimSpace(jobID) == "" {
		return flowdomain.StepFailed, true
	}
	item, found, err := h.jobStore.GetByID(jobID)
	if err != nil {
		return "", false
	}
	if !found {
		return flowdomain.StepFailed, true
	}
	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case "succeeded", "completed":
		return flowdomain.StepSucceeded, true
	case "failed", "error":
		return flowdomain.StepFailed, true
	default:
		return "", false
	}
}

// launchFlowStep lance une etape comme n'importe quel job.
//
// Par le meme chemin exactement : les montages, le profil de sortie reseau,
// la resolution des droits et la provenance sont ceux d'un job ordinaire. Une
// etape lancee par un chemin a elle divergerait, et le jour ou ca se verrait
// serait celui ou un resultat ne s'expliquerait plus.
func (h Handlers) launchFlowStep(ctx context.Context, run flowdomain.Run, definition flowdomain.Flow, key string) (string, error) {
	var step flowdomain.Step
	for _, candidate := range definition.Steps {
		if candidate.Key == key {
			step = candidate
		}
	}
	name := strings.TrimSpace(step.Name)
	if name == "" {
		name = definition.Name + " / " + key
	}

	// Le meme chemin que n'importe quel lancement, appele hors HTTP.
	//
	// launchJob rend l'enregistrement en plus d'ecrire sa reponse : c'est ce
	// qui permet a l'ordonnanceur de l'emprunter sans ecrire un second
	// lanceur. Le collecteur jette la reponse ; l'identite est celle de la
	// personne qui a demarre le run, parce qu'un flow ne doit pas donner
	// acces a plus que ce qu'elle peut lire.
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/jobs", nil)
	if err != nil {
		return "", err
	}
	collecteur := &discardWriter{}
	record, created := h.launchJob(collecteur, request,
		auth.Identity{Subject: run.StartedBy, Username: run.StartedBy},
		run.StartedBy,
		createJobRequest{
			ProjectID:    run.ProjectID,
			Name:         name,
			Image:        step.Image,
			Command:      step.Command,
			Args:         step.Args,
			HardwareTier: step.HardwareTier,
		}, "")
	if !created {
		return "", fmt.Errorf("the step could not be launched: %s", strings.TrimSpace(string(collecteur.body)))
	}
	return record.ID, nil
}

// discardWriter recoit la reponse HTTP que launchJob ecrit quand personne ne
// la lit. Le corps est garde : c'est lui qui porte la raison d'un refus, et
// « l'etape n'a pas pu demarrer » sans la raison ferait recommencer le
// diagnostic a zero.
type discardWriter struct {
	status int
	body   []byte
}

func (d *discardWriter) Header() http.Header { return http.Header{} }
func (d *discardWriter) Write(payload []byte) (int, error) {
	d.body = append(d.body, payload...)
	return len(payload), nil
}
func (d *discardWriter) WriteHeader(status int) { d.status = status }
