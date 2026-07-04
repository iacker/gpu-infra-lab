# Runbook — Nœud GPU à drainer (maintenance / dégradation)

**Contexte.** Il faut sortir un nœud GPU du service (patch driver, XID errors,
température, maintenance Scaleway) sans casser le SLO de disponibilité.

## Détection d'une dégradation nœud
- DCGM `DCGM_FI_DEV_XID_ERRORS` != 0 → erreur GPU matérielle/driver.
- `DCGM_FI_DEV_GPU_TEMP` > seuil constructeur soutenu.
- `nvidia-smi` : `ERR!` ou ECC non corrigeables.

## Procédure de drain propre
1. `kubectl cordon <node>` — stoppe le scheduling.
2. Laisser les requêtes en vol se terminer (drain gracieux du service vLLM) :
   `kubectl drain <node> --ignore-daemonsets --delete-emptydir-data --grace-period=60`.
3. Kueue réadmet les workloads en attente sur la capacité restante ; si mono-nœud,
   la LocalQueue met en attente → **c'est le comportement voulu**, pas une panne.
4. Maintenance / reboot / patch driver via le playbook Ansible (idempotent).
5. `kubectl uncordon <node>` une fois `nvidia-smi` sain et node `Ready`.

## Garde-fou SLO
- Sur un lab mono-GPU, un drain = indisponibilité assumée : le documenter comme
  **limite connue** et décrire la mitigation multi-nœud (2e ResourceFlavor, PodDisruptionBudget).
- Ne jamais draîner sans avoir vérifié qu'aucun bench critique n'écrit en base à cet instant.

## Vérification
- Node `Ready`, `XID_ERRORS=0`, pod vLLM `Running`, 1 requête de fumée OK via `bench.py --n 1`.
