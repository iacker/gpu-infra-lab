# Runbook — Saturation du KV cache (OOM GPU en inférence)

**Symptôme.** Latence P99 qui explose puis requêtes en échec `500` ; DCGM
`DCGM_FI_DEV_FB_USED` proche de la capacité carte ; logs vLLM `CUDA out of memory`
ou `Preemption` répétés.

## Détection
- Alerte Prometheus : `DCGM_FI_DEV_FB_USED / DCGM_FI_DEV_FB_TOTAL > 0.95` pendant 2 min.
- Dashboard Grafana « GPU Inference » → panneau *KV cache / FB used*.

## Diagnostic (3 min)
1. `kubectl logs deploy/vllm | grep -iE 'oom|preempt|kv'` — confirmer préemptions.
2. Vérifier la charge : `sum(rate(vllm:num_requests_running[1m]))` — pic de concurrence ?
3. Vérifier `--max-model-len` vs longueur réelle des prompts (contexte trop long ?).

## Remédiation
- **Immédiat** : baisser la concurrence admise via Kueue (réduire `nominalQuota` cpu/gpu
  de la ClusterQueue) ou `--max-num-seqs` sur vLLM → moins de séquences concurrentes.
- **Court terme** : activer/renforcer la quantif (`--quantization fp8`) → plus de place KV.
- **Structurel** : baisser `--gpu-memory-utilization` marge de sécurité, ou passer sur un
  GPU à plus grande VRAM. Documenter le nouveau plafond de concurrence dans le SLO.

## Vérification post-incident
- FB used redescend < 85 %, P99 sous SLO, zéro préemption sur 10 min.
- Ajouter un test de charge de non-régression au harness (`bench.py --n <pic observé>`).
