#!/usr/bin/env bash
# burst.sh — FinOps du lab : ne payer la L4 que les heures réellement utilisées.
#
# Scaleway facture l'instance à l'heure TANT QU'ELLE EXISTE et tourne.
# `off` archive l'instance (poweroff) : GPU/CPU/RAM ne sont plus facturés,
# seuls le volume bloc (~qq centimes/jour) et l'IP flexible restent.
# `on` la redémarre : k3s et toute la stack reviennent seuls (systemd),
# le modèle est déjà dans le cache disque (/var/lib/vllm-cache) -> ~3 min pour être servable.
#
# Prérequis : scw CLI configurée (`scw init`, une fois).
set -euo pipefail

# Renseigner via l'environnement (voir ansible/inventory.ini pour l'IP du nœud) :
#   export SCW_INSTANCE_ID=<id-instance>  GPU_NODE_IP=<ip-publique>
INSTANCE_ID="${SCW_INSTANCE_ID:?exporter SCW_INSTANCE_ID = id de instance Scaleway}"
ZONE="${GPU_LAB_ZONE:-fr-par-1}"

usage() { echo "usage: $0 {status|off|on|watch}"; exit 1; }
[ $# -ge 1 ] || usage

state() { scw instance server get "$INSTANCE_ID" zone="$ZONE" -o json | jq -r .state; }

case "$1" in
  status)
    scw instance server get "$INSTANCE_ID" zone="$ZONE" -o json \
      | jq -r '{name, state, public_ip: .public_ip.address, type: .commercial_type}'
    ;;
  off)
    echo "[burst] arrêt propre de la stack (drain des requêtes en cours)…"
    ssh -o ConnectTimeout=10 "root@${GPU_NODE_IP:?exporter GPU_NODE_IP = ip publique du noeud}" \
      'k3s kubectl scale deploy/vllm-mistral --replicas=0 --timeout=60s || true' || true
    echo "[burst] poweroff + archivage (fin de la facturation GPU)…"
    scw instance server stop "$INSTANCE_ID" zone="$ZONE" --wait
    echo "[burst] instance archivée. Facturation restante : volume bloc + IP flexible."
    ;;
  on)
    echo "[burst] démarrage…"
    scw instance server start "$INSTANCE_ID" zone="$ZONE" --wait
    IP=$(scw instance server get "$INSTANCE_ID" zone="$ZONE" -o json | jq -r .public_ip.address)
    echo "[burst] up, IP: $IP — attente SSH…"
    until ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new "root@$IP" true 2>/dev/null; do sleep 5; done
    echo "[burst] attente k3s Ready…"
    ssh "root@$IP" 'until k3s kubectl wait --for=condition=Ready node --all --timeout=10s 2>/dev/null; do sleep 5; done'
    ssh "root@$IP" 'k3s kubectl scale deploy/vllm-mistral --replicas=1'
    echo "[burst] stack relancée. vLLM servable dans ~2-3 min (modèle en cache disque)."
    ;;
  watch)
    watch -n 10 "scw instance server get $INSTANCE_ID zone=$ZONE -o json | jq -r .state"
    ;;
  *) usage ;;
esac
