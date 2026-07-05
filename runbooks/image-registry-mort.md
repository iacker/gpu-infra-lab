# Runbook — ImagePullBackOff sur registre disparu

> Vécu en réel le 2026-07-05 : Kueue v0.8.1, sidecar `kube-rbac-proxy` pointant sur
> `gcr.io/kubebuilder/*` — registre définitivement fermé par Google.

**Symptôme.** Pod bloqué `ImagePullBackOff` ou `ErrImagePull` ; le déploiement ne devient
jamais Available ; `kubectl wait` timeout.

## Détection
- Alerte kube-state-metrics `KubePodNotReady` / rollout qui timeout en CI.
- `kubectl get pods -A | grep -v Running` en fin de déploiement (gate du playbook).

## Diagnostic (2 min)
1. `kubectl describe pod <pod> | grep -A5 Events` — identifier QUELLE image échoue.
   Piège vécu : dans un pod multi-conteneurs, le principal peut être `Running` pendant
   que le **sidecar** boucle (Kueue : manager OK, kube-rbac-proxy KO).
2. Tester le pull hors cluster : `crictl pull <image>` ou `curl -I https://<registre>/v2/`.
3. Distinguer : registre mort (404/NXDOMAIN définitif) vs rate-limit (429, transitoire)
   vs auth (401, imagePullSecret).

## Remédiation
- **Registre mort** : chercher si le projet upstream a migré l'image (release notes).
  Cas Kueue : versions ≥ v0.9 ont supprimé le sidecar → upgrade v0.10.1. Sinon, re-héberger
  l'image sur son propre registre et patcher le manifest.
- **Rate-limit** : imagePullSecret avec compte authentifié, ou miroir local.
- **Auth** : recréer l'imagePullSecret, vérifier l'expiration du token.

## Prévention
- Pinner les versions ET surveiller les annonces de dépréciation des registres publics
  (gcr.io/kubebuilder, k8s.gcr.io → registry.k8s.io : l'histoire se répète).
- Air-gap/souverain : miroir de registre local (Harbor/Zot), aucune dépendance à un
  registre public au moment du déploiement — argument fort pour une infra souveraine.
