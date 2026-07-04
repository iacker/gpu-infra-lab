# Méthodologie énergie — DCGM (mesure) vs Kepler (estimation)

Ce lab vend un récit « coût + énergie maîtrisés ». La crédibilité tient à **une** règle :
ne jamais présenter une estimation comme une mesure.

## Source de vérité GPU : DCGM
- `DCGM_FI_DEV_POWER_USAGE` = puissance instantanée de la carte en watts, lue par le
  télémètre NVIDIA. C'est une **mesure** matérielle.
- Énergie sur une fenêtre : intégrale de la puissance dans le temps
  `∫ P dt` ≈ `avg_over_time(DCGM_FI_DEV_POWER_USAGE[w]) * w`.
- J/1k tokens = (watts / tokens_par_seconde) * 1000 — voir l'exporter Go `lab_joules_per_1k_tokens`.

## Kepler : à quoi il sert ici, et à quoi il ne sert PAS
- Kepler estime l'énergie par pod/nœud. Sur une **instance cloud virtualisée** (le cas
  Scaleway), il n'a souvent **pas accès aux compteurs RAPL** ni à la télémétrie GPU brute,
  et retombe sur un **modèle**. Le chiffre peut être du bruit.
- Usage autorisé dans ce lab : récit d'efficacité **node/CPU**, comparaison relative,
  toujours **étiqueté « estimation »** sur le dashboard.
- Usage interdit : sortir un « J/token GPU » depuis Kepler. Pour le GPU, c'est DCGM, point.

## Ce que ça prouve en entretien
La capacité à distinguer mesure et estimation, et à choisir la bonne source par métrique,
est exactement la rigueur attendue d'un SRE qui pose des SLO énergie. C'est un argument,
pas un détail.
