# Runbook — vLLM down (endpoint d'inférence injoignable)

> Vécu en réel le 2026-07-05 : injection de panne (kill du pod sous charge).
> Timeline mesurée : kill à T+0 → détection dashboard < 10 s → pod recréé automatiquement → service rétabli à T+4 min (rechargement modèle depuis le cache disque).

**Symptôme.** Requêtes API en échec (connexion refusée / 503). Dashboard : GPU util
tombe à 0 %, puissance DCGM chute à ~16 W (idle), débit tokens à zéro.

## Détection
- Alerte `VLLMDown` : `absent(up{service="vllm-mistral"} == 1)` pendant 2 min → Telegram (severity: critical).
- Grafana « GPU Inference Reliability Lab » : panels GPU util / power / throughput s'effondrent simultanément.

## Diagnostic (2 min)
1. `kubectl get pods -l app=vllm-mistral` — état du pod :
   - `CrashLoopBackOff` → voir logs (étape 2)
   - `Pending` → problème de ressource GPU (quota Kueue ? nœud ?)
   - `ContainerCreating` long → pull d'image ou volume
2. `kubectl logs -l app=vllm-mistral --tail=50` — signatures connues :
   - `Failed to infer device type` → le conteneur ne voit pas le GPU → vérifier `runtimeClassName: nvidia` dans le spec (vécu, voir ci-dessous)
   - `CUDA out of memory` → runbook kv-cache-saturation.md
   - `401/403 Hugging Face` → secret `hf-token` absent/expiré
3. `kubectl describe pod -l app=vllm-mistral | tail -20` — events (image pull, scheduling).

## Remédiation
- **Pod tué / nœud OOM** : rien à faire — le Deployment recrée le pod ; modèle rechargé depuis
  `/var/lib/vllm-cache` (hostPath) en ~3-4 min. Vérifier le retour via l'alerte resolved.
- **`Failed to infer device type`** : ajouter `runtimeClassName: nvidia` au pod spec.
  Cause : sur k3s + GPU Operator, la RuntimeClass nvidia n'est pas le runtime par défaut ;
  le device plugin réserve le GPU mais n'injecte pas /dev/nvidia*.
- **Rollout bloqué (nouveau pod Pending, ancien Running)** : deadlock RollingUpdate sur
  single-GPU — le nouveau pod attend le GPU tenu par l'ancien. Fix : `strategy: Recreate`
  (déjà dans le manifest). Débloquer : `kubectl delete pod <ancien>`.
- **Secret HF manquant** : `kubectl create secret generic hf-token --from-literal=token=***`.

## Vérification post-incident
- `curl vllm-mistral:8000/v1/models` répond ; alerte VLLMDown resolved dans Telegram.
- Dashboard : util/power/débit revenus au niveau pré-incident.
- Noter la durée d'indisponibilité réelle et la comparer au SLO (budget d'erreur).

## Leçon structurelle (mesurée)
Sur ce lab, **Kubernetes s'est auto-réparé (4 min) plus vite que la fenêtre d'alerte (2 min
d'absence + délais de groupe)** : l'astreinte reçoit l'alerte alors que la remédiation est
déjà en cours. C'est le comportement cible — l'alerte sert de trace, pas de réveil, tant que
le rechargement modèle reste sous le SLO d'indisponibilité.
