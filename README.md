<p align="center">
  <img src="assets/logo.png" alt="GPU Inference Reliability Lab" width="200">
</p>

<h1 align="center">GPU Inference Reliability Lab</h1>

> Servir un LLM français sur un cloud français, et **prouver** qu'on sait l'observer, le
> fiabiliser et en maîtriser le coût (€) et l'énergie (J) — au token près, au watt près.

Mistral-7B servi par vLLM sur Kubernetes (k3s), sur une instance GPU **Scaleway L4**.
Tout est provisionné par Ansible, mesuré par Prometheus/DCGM, et cassé exprès pour
démontrer la boucle complète du SRE : **détection → diagnostic → remédiation → runbook**.

Déployer un modèle sur un GPU, c'est un tutoriel. L'opérer, c'est un métier.
Ce dépôt documente le métier.

## Résultats mesurés (pas estimés)

Premiers chiffres sur L4 24 GB, Mistral-7B-Instruct-v0.3, vLLM v0.6.6 (2026-07-05) :

| Métrique | FP16 | FP8 | Δ |
|---|---|---|---|
| Débit single-stream (tok/s) | 17,4 | 28,3 | **+63 %** |
| Latence P50 (s) | 6,33 | 3,60 | −43 % |
| Puissance moyenne (W, DCGM) | 69,9 | 56,9 | −19 % |
| **Énergie (J / 1k tokens)** | **4 017** | **2 032** | **−49 %** |
| Coût (€ / 1M tokens, L4 à 0,75 €/h) | 11,97 | 7,36 | −39 % |

Détails et protocole : [`docs/bench-fp16-vs-fp8.md`](docs/bench-fp16-vs-fp8.md).
Méthodologie énergie (mesure DCGM vs estimation) : [`docs/energy-methodology.md`](docs/energy-methodology.md).

## Architecture

### Diagramme d'architecture

```mermaid
flowchart TD
    A["Ansible · 2 playbooks idempotents"] --> K["k3s + NVIDIA GPU Operator<br/>(Scaleway L4)"]
    K --> V["vLLM · Mistral-7B<br/>API OpenAI-compatible"]
    K --> Q["Kueue v0.10.1<br/>ResourceFlavor → ClusterQueue → LocalQueue"]
    K --> D["DCGM exporter<br/>puissance carte RÉELLE = vérité énergie"]
    K --> P["kube-prometheus-stack<br/>scrape 5s · Grafana 14 panels · Loki"]

    Q -->|admission ADMITTED| B["Jobs de bench<br/>latence · débit · coût"]
    B --> M["MariaDB · registre bench_runs"]

    V -->|métriques vllm:*| P
    D -->|watts| P
    C["cost-exporter (Go)<br/>€/token · J/1k tokens"] --> P
    P --> AL["Alertmanager · 5 règles SLO"] -->|astreinte| T["Telegram"]
```

## Ce que le lab démontre

Les incidents sont réels : cinq [runbooks](runbooks/) tirés de pannes provoquées ou subies —
pod GPU tué sous charge, registre d'images mort, piège Helm `--reuse-values`, saturation du
KV cache, [GPU Operator cassé après un reboot](runbooks/reboot-driver-mount.md). Pour les voir
venir, un [dashboard Grafana](dashboards/) de 14 panels compte chaque token deux fois (API et
Prometheus) et des [règles SLO](manifests/alerting/) partent en astreinte sur Telegram (vLLM
down, P99 > 5 s, KV cache saturé, GPU au cap, température).

Le reste de la stack rend ces mesures possibles : vLLM sur L4 avec comparatif FP16/FP8 mesuré
au watt, provisionné par [Ansible](ansible/) en deux playbooks relançables, et une
[CI GitLab](.gitlab-ci.yml) qui lint, build, déploie puis lance les benchs via
[Kueue](manifests/kueue/). Ces benchs sont des Jobs admis par la file, écrits en Go et Python
([exporter/](exporter/), [bench/](bench/)) et archivés dans [MariaDB](bench/schema.sql). Le
volet [réseau](docs/reseau.md) va du tunnel SSH au design BGP/IPv6/InfiniBand multi-nœud, et le
[FinOps](scripts/burst.sh) loue le GPU à l'heure, allumé par rafales.

## Incident vécu (résumé)

Kill du pod vLLM en pleine charge (injection de panne, 2026-07-05) :
détection sur dashboard < 10 s (GPU 99 % → 0 %, 72 W → 17 W) · alerte Telegram armée ·
**Kubernetes a recréé le pod et rechargé le modèle en ~4 min, sans intervention**.
Timeline complète et leçons : [`runbooks/vllm-down.md`](runbooks/vllm-down.md).

## Démarrage

```bash
# 1. Une instance GPU Scaleway (L4, image "GPU OS passthrough") + son IP dans l'inventaire
cp ansible/inventory.ini.example ansible/inventory.ini   # renseigner l'IP

# 2. Fondation : k3s + GPU Operator
ansible-playbook -i ansible/inventory.ini ansible/playbook.yml

# 3. Plateforme : Kueue + monitoring + alerting + vLLM (+ token HF pour Mistral)
ansible-playbook -i ansible/inventory.ini ansible/apps.yml -e hf_token=hf_***

# 4. Accès (rien n'est exposé publiquement — tunnels SSH)
ssh -L 3000:localhost:3000 root@<IP> 'kubectl -n monitoring port-forward svc/kps-grafana 3000:80'
# -> http://localhost:3000, dashboard "GPU Inference Reliability Lab"

# 5. Un bench admis par Kueue, résultats en MariaDB
kubectl apply -f manifests/bench/job.yaml
```

Secrets attendus (jamais dans le dépôt) : `hf-token` (Hugging Face, modèle gated),
`telegram-bot-token` (alerting), `mariadb-creds` (registre bench).

## Garde-fou méthodologique

**DCGM = mesure** (télémétrie physique de la carte, watts réels).
Les outils qui *estiment* l'énergie par modèle statistique sont utiles mais ne sont pas
des mesures, et sur du virtualisé ils n'ont souvent pas accès aux vrais compteurs.
Ici, chaque chiffre publié dit de quelle famille il vient. Vendre une estimation comme
une mesure, c'est exactement le flou que ce lab s'interdit — y compris quand le chiffre
estimé serait plus flatteur (cf. la limite qualité non mesurée du bench FP8).

## Coût du lab

GPU L4 Scaleway loué à l'heure (~0,75 €/h), allumé par rafales (`scripts/burst.sh`) →
la totalité des mesures de ce dépôt a coûté quelques euros.

## Contexte

Lab construit comme preuve opérationnelle pour [GPUInfraSystems](https://gpuinfrasystems.com)
— observabilité et fiabilité d'inférence GPU sur infrastructure souveraine européenne.
Modèle 🇫🇷 (Mistral), cloud 🇫🇷 (Scaleway), stack 100 % open source.
