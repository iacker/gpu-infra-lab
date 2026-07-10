# Chapitre réseau — du paquet TCP au token

Comment une requête d'inférence traverse le lab, couche par couche, et les choix réseau
qui vont avec. (Répond au volet « TCP/IP, DNS, BGP, load-balancing, IPv6 » de l'offre.)

## Topologie actuelle (lab single-node)

```
Client (Mac)
  │  SSH -L (tunnel chiffré, port-forward local)
  ▼
Instance Scaleway L4 — IPv4 publique <IP_PUBLIQUE> /  IPv6 <PREFIXE_IPV6>/64
  │  aucun port ouvert sauf 22 (surface d'attaque minimale)
  ▼
k3s : CNI flannel (VXLAN) — pods en 10.42.0.0/16, services en 10.43.0.0/16
  │  kube-proxy (iptables) : ClusterIP -> pod
  ▼
Service vllm-mistral:8000 (ClusterIP) -> pod vLLM
CoreDNS : résolution *.svc.cluster.local (ex: mariadb.default.svc.cluster.local)
```

### Choix et justifications

| Choix | Pourquoi |
|---|---|
| **Aucune exposition publique** (pas de LoadBalancer/Ingress) | Un endpoint LLM non authentifié sur internet = abus garanti (scan + génération gratuite). Tunnel SSH = chiffrement + authentification par clé, zéro surface. |
| ClusterIP partout | Les services ne se parlent qu'en interne ; DNS de cluster (CoreDNS) comme seul annuaire. |
| Traefik k3s désactivé | Pas d'ingress inutile ; on ajoute un LB *quand* on expose, pas avant. |

## Passage à l'échelle (multi-nœud / prod) — le design

1. **Exposition** : LoadBalancer Scaleway devant un Ingress (TLS terminé au LB, cert ACME),
   avec authentification (clé API / OIDC) AVANT tout endpoint LLM public.
2. **IPv6** : l'instance a déjà une /64 ; en prod, dual-stack k8s (`ipFamilyPolicy:
   PreferDualStack`) — l'inférence est un service nord-sud parfait pour IPv6-first.
3. **BGP** : sur un parc GPU multi-rack, annonce des VIP de service en BGP (MetalLB mode
   BGP / Cilium BGP control plane) vers les ToR — pas de SPOF LB, ECMP naturel.
4. **Est-ouest GPU** : pour du multi-nœud NCCL (entraînement/inférence distribuée),
   fabric dédiée InfiniBand/RoCE, séparée du réseau de service — la latence NCCL ne doit
   jamais partager la file avec le trafic API.
5. **DNS** : split-horizon — noms internes (cluster.local) jamais exposés ; publics gérés
   au niveau du LB.

## Debug réseau type (boîte à outils)

```bash
kubectl exec -it <pod> -- getent hosts mariadb          # résolution CoreDNS
kubectl get endpointslices -l kubernetes.io/service-name=vllm-mistral   # service -> pods
ss -tlnp | grep 8000                                     # écoute effective dans le pod
curl -s -o /dev/null -w '%{time_connect} %{time_starttransfer}\n' http://vllm-mistral:8000/health
                                                         # TCP connect vs TTFB : réseau ou modèle ?
```

Le dernier point est la question réseau centrale en inférence : **si `time_connect` est
bas mais `time_starttransfer` haut, le réseau est innocent** — la latence est dans le
modèle (file d'attente, KV cache, batch). Mesurer avant d'accuser.
