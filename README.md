# GPU Inference Reliability Lab

> Servir un LLM sur Kubernetes et **prouver** qu'on sait l'observer, le fiabiliser et
> en maîtriser le coût (€) et l'énergie (J). Reproductible, public, tournant sur GPU souverain.

Artefact de candidature **SRE — AI GPU Clusters (Scaleway)** et preuve opérationnelle de
[GPUInfraSystems](https://gpuinfrasystems.com). Observabilité et remédiation d'incidents en
épine dorsale, pas en option.

## Ce que le lab démontre

| Exigence poste | Preuve dans ce repo |
|---|---|
| Observabilité (Prometheus/Grafana/Elastic) | `dashboards/` + DCGM + Loki |
| Monitoring → diagnostic → **remédiation** + on-call | `runbooks/` + injection de pannes |
| GPU & HPC | vLLM sur GPU, DCGM, scheduling multi-tenant Kueue |
| IaC (Ansible) | `ansible/` — provisioning nœud + k3s + GPU Operator |
| CI/CD (GitLab) | `.gitlab-ci.yml` — lint, build, deploy, bench |
| Python / Go | `bench/` (harness Python) + `exporter/` (coût/token en Go) |
| MariaDB | `bench/schema.sql` — registre des runs |
| Souverain / Scaleway | Déployé sur instance GPU Scaleway L4 |

## Architecture

```
Ansible ──> k3s + NVIDIA GPU Operator
                │
                ├─ Kueue        (admission, quota GPU, multi-tenant)
                ├─ vLLM          (Mistral-7B / Llama-3-8B)
                ├─ DCGM exporter (puissance GPU RÉELLE — source de vérité énergie)
                └─ Prometheus + Grafana + Loki
                        │
             bench/ (Python) ──> MariaDB (registre runs)
             exporter/ (Go)  ──> métrique €/token
```

## ⚠️ Garde-fou énergie

**DCGM = mesure GPU réelle** (`DCGM_FI_DEV_POWER_USAGE`, watts carte). C'est la source de vérité
de tout le récit énergie du lab. Aucune estimation modélisée n'est présentée comme une mesure —
la rigueur mesure/estimation est l'argument, pas un détail.

## Quickstart

```bash
# 1. Provision (depuis ton poste, vers l'instance Scaleway)
cd ansible && cp inventory.ini.example inventory.ini   # remplir l'IP
ansible-playbook -i inventory.ini playbook.yml

# 2. Plateforme observabilité + serving (sur le nœud, kubeconfig en place)
kubectl apply -f manifests/kueue/
helm upgrade --install vllm ... -f helm/values-vllm.yaml     # voir helm/README
helm upgrade --install dcgm ... -f helm/values-dcgm.yaml

# 3. Bench
cd bench && pip install -r requirements.txt
python bench.py --endpoint http://<vllm-svc>:8000 --run-id demo-01

# 4. Exporter €/token
cd exporter && go build -o cost-exporter . && ./cost-exporter
```

## Statut

Squelette scaffoldé — chaque dossier a un `TODO.md` ou des commentaires marquant ce qui reste
à câbler sur le vrai matériel. Rien ici n'est fabriqué : les valeurs de bench/dashboards se
remplissent au premier run réel sur GPU.

## Coût

GPU L4 Scaleway loué à l'heure, lancé par rafales pour les benchmarks → quelques dizaines d'€.
Vérifier la grille tarifaire Scaleway avant de lancer.
