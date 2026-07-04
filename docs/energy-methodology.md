# Méthodologie énergie — DCGM (mesure)

Ce lab vend un récit « coût + énergie maîtrisés ». La crédibilité tient à **une** règle :
ne jamais présenter une estimation comme une mesure. Donc une seule source, matérielle.

## Source de vérité GPU : DCGM
- `DCGM_FI_DEV_POWER_USAGE` = puissance instantanée de la carte en watts, lue par le
  télémètre NVIDIA. C'est une **mesure** matérielle, pas un modèle.
- Énergie sur une fenêtre : intégrale de la puissance dans le temps
  `∫ P dt` ≈ `avg_over_time(DCGM_FI_DEV_POWER_USAGE[w]) * w`.
- J/1k tokens = (watts / tokens_par_seconde) * 1000 — voir l'exporter Go `lab_joules_per_1k_tokens`.

## Pourquoi pas d'estimateur logiciel (type Kepler)
Sur une instance cloud virtualisée (le cas Scaleway), un estimateur logiciel n'a souvent pas
accès aux compteurs RAPL ni à la télémétrie GPU brute et retombe sur un modèle — le chiffre
peut être du bruit. Un « J/token GPU » ne se dérive que de DCGM. Point.

## Ce que ça prouve en entretien
La capacité à choisir la bonne source par métrique — et à refuser une estimation déguisée
en mesure — est exactement la rigueur attendue d'un SRE qui pose des SLO énergie.
