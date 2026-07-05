# Runbook — Upgrade Helm qui casse (`nil pointer evaluating ...`)

> Vécu en réel le 2026-07-05 : `helm upgrade gpu-operator --reuse-values` sans `--version`
> → `nil pointer evaluating interface {}.enabled` (template du chart latest lisant des
> clés absentes des values de la release v24.6.1).

**Symptôme.** `helm upgrade` échoue au rendu des templates avec une erreur `nil pointer`
ou `map has no entry for key`, alors que la commande semble anodine (un simple `--set`).

## Détection
- Échec immédiat de la commande (rc≠0) — la release n'est PAS modifiée (le rendu échoue
  avant l'apply). Vérifier : `helm history <release>`.

## Diagnostic (1 min)
1. `helm list -n <ns>` — quelle version de chart est installée ?
2. La commande d'upgrade pinne-t-elle `--version` ? Si non → c'est ça.
   `--reuse-values` gèle les values de l'ancienne version pendant que le chart latest
   attend de nouvelles clés : combinaison structurellement fragile.

## Remédiation
- Repincer la version installée : `helm upgrade ... --reuse-values --version <installée>`.
- Pour monter de version de chart : NE PAS utiliser `--reuse-values` ; regénérer des
  values complètes (`helm get values` + merge manuel avec les nouveaux défauts).

## Prévention
- Règle d'or : **`--reuse-values` et `--version` vont toujours par paire.**
- En IaC : la version du chart est une variable du playbook, jamais implicite (appliqué
  dans `ansible/apps.yml`).
