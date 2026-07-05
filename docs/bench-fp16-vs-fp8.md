# FP16 vs FP8 sur L4 : le comparatif mesuré au watt

**Date** : 2026-07-05 · **Matériel** : NVIDIA L4 24 GB (Scaleway L4-1-24G, PAR1) · **Modèle** : Mistral-7B-Instruct-v0.3 · **Serveur** : vLLM v0.6.6

## Protocole

- Harness `bench/bench.py` : 30 requêtes séquentielles, `max_tokens=128`, même prompt.
- FP8 : quantification dynamique vLLM (`--quantization=fp8`), même checkpoint.
- Énergie : **mesurée** par DCGM (`DCGM_FI_DEV_POWER_USAGE`, moyenne sur la fenêtre exacte du run via PromQL `avg_over_time(...[Ns] @ t1)`), tokens comptés par `increase(vllm:generation_tokens_total[Ns] @ t1)`. Aucune estimation.
- Coût : grille L4 à 0,75 €/h (à revérifier sur la grille Scaleway du moment).

## Résultats

| Métrique | FP16 (baseline) | FP8 | Δ |
|---|---|---|---|
| Débit (tokens/s, single-stream) | 17,4 | 28,3 | **+63 %** |
| Latence P50 (s) | 6,33 | 3,60 | **−43 %** |
| Latence P99 (s) | 7,39 | 4,73 | −36 % |
| Puissance moyenne (W, DCGM) | 69,9 | 56,9 | −19 % |
| **Énergie (J / 1k tokens)** | **4 017** | **2 032** | **−49 %** |
| Coût (€ / 1M tokens) | 11,97 | 7,36 | −39 % |
| VRAM occupée (GB) | 19,9 | 19,9 | = (préallocation vLLM) |

Fenêtres brutes : FP16 = 190 s, 69,90 W moy, 3 306 tokens · FP8 = 110 s, 56,93 W moy, 3 082 tokens.

## Lectures

1. **L'énergie par token est divisée par 2.** Double effet : le GPU finit plus vite (moins de secondes par token) ET tire moins de watts en FP8 (56,9 vs 69,9 W — les tensor cores FP8 de l'architecture Ada travaillent moins longtemps par opération).
2. **La VRAM ne bouge pas — et c'est normal.** vLLM préalloue 90 % de la carte quoi qu'il arrive. Ce qui change : les poids passent de ~14 GB à ~7 GB, donc le **KV cache disponible double**. Sous forte concurrence, FP8 tiendra beaucoup plus de requêtes simultanées avant saturation.
3. **Ce qu'on n'a PAS mesuré : la qualité.** La quantification dégrade (légèrement) les sorties. Sans éval de qualité (perplexité, benchmark de tâches), ces chiffres ne suffisent pas à décider « FP8 en prod ». Le manifest committé reste donc en FP16 par défaut ; FP8 s'active par un flag documenté. Dire ça, c'est le même réflexe que DCGM-vs-estimation : on ne vend pas un chiffre au-delà de ce qu'il prouve.
4. Single-stream séquentiel = borne basse du débit. En continuous batching concurrent, l'écart FP8/FP16 se creuse encore (KV cache ×2). À mesurer en J4-concurrence.

## Reproduire

```bash
# FP16 (défaut du repo)
python3 bench/bench.py --endpoint http://localhost:8000 --run-id fp16 --n 30 --max-tokens 128 --gpu-eur-hour 0.75
# Passer en FP8
kubectl patch deploy vllm-mistral --type=json \
  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--quantization=fp8"}]'
# Revenir au défaut FP16
kubectl apply -f manifests/vllm/vllm.yaml
# Énergie de la fenêtre [T0;T1] (PromQL)
avg_over_time(DCGM_FI_DEV_POWER_USAGE[<T1-T0>s] @ <T1>)
increase(vllm:generation_tokens_total[<T1-T0>s] @ <T1>)
```
