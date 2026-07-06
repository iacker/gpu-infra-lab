# Runbook — GPU Operator cassé après un redémarrage du nœud

**Contexte.** Après un `reboot` (maintenance, coupure, redémarrage manuel de l'instance
Scaleway), tout le namespace `gpu-operator` part en `CrashLoopBackOff` /
`RunContainerError`, alors que `nvidia-smi` fonctionne parfaitement sur l'hôte.

## Symptôme

```
kubectl -n gpu-operator get pods
# nvidia-device-plugin-daemonset   0/1  CrashLoopBackOff
# nvidia-dcgm-exporter             0/1  CrashLoopBackOff
# gpu-feature-discovery            0/1  CrashLoopBackOff
# nvidia-cuda-validator            0/1  Init:CrashLoopBackOff
```

Message dans les events du pod :

```
OCI runtime create failed: ... nvidia-container-cli.real: initialization error:
load library failed: libnvidia-ml.so.1: cannot open shared object file: no such
file or directory: unknown
```

## Cause racine

Ce lab tourne avec `driver.enabled=false` dans le Helm chart GPU Operator (driver
NVIDIA posé nativement sur l'hôte via `ubuntu-drivers autoinstall`, cf.
[`ansible/playbook.yml`](../ansible/playbook.yml) — pas de driver-container géré
par l'opérateur).

`nvidia-container-cli` (utilisé par le hook OCI de tous les pods GPU) est
configuré par le toolkit de l'opérateur avec `root = "/run/nvidia/driver"` — un
chemin qui doit exposer les libs driver de l'hôte. Mais `/run` est un **tmpfs**,
vidé à chaque redémarrage. Le bind-mount qui rend le driver hôte visible à cet
endroit n'est **pas recréé automatiquement** par le GPU Operator quand
`driver.enabled=false` — il n'y a pas de driver-container pour le faire.

Résultat : après reboot, `/run/nvidia/driver` est un répertoire vide, et tout
conteneur qui demande le GPU échoue à l'init du hook OCI.

## Détection

```bash
ls -la /run/nvidia/driver/          # vide (pas de sbin/, pas de usr/)
kubectl -n gpu-operator get pods    # CrashLoopBackOff / RunContainerError
nvidia-smi                          # fonctionne (le host n'est pas en cause)
```

## Remédiation immédiate

```bash
mount --rbind / /run/nvidia/driver
mount --make-rshared /run/nvidia/driver
# puis forcer la ré-init des pods bloqués :
kubectl -n gpu-operator delete pod --all
```

## Fix permanent (déjà appliqué)

Une unit systemd (`nvidia-driver-root-bind.service`, `Before=k3s.service`) fait
ce bind-mount à chaque boot, avant que k3s ne démarre. Elle est posée de façon
idempotente par [`ansible/playbook.yml`](../ansible/playbook.yml) (tâche
*"Persist /run/nvidia/driver bind-mount"*) — donc présente dès la prochaine
réinstallation from scratch, pas seulement sur ce nœud.

```bash
systemctl is-enabled nvidia-driver-root-bind.service   # enabled
systemctl status nvidia-driver-root-bind.service
```

## Vérification

- `ls /run/nvidia/driver/usr/lib/x86_64-linux-gnu/ | grep nvidia-ml` → présent.
- `kubectl -n gpu-operator get pods` → tout `Running`/`Completed`.
- `kubectl get nodes -o jsonpath='{.items[0].status.allocatable.nvidia\.com/gpu}'` → `1`.
- Requête de fumée sur vLLM (`curl .../v1/completions`) → réponse cohérente.

## Garde-fou

- Si une future version du chart GPU Operator change le comportement de
  `driver.enabled=false` (ou passe à `useNvidiaDriverCRD`/driver-container géré),
  revalider que ce bind-mount manuel est toujours nécessaire avant de le
  reconduire tel quel.
