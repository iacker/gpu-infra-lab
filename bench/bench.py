#!/usr/bin/env python3
"""Harness de bench vLLM -> métriques latence/débit/coût -> MariaDB.

Mesure au token près : TTFT, latence P50/P99, tokens/s, €/1M tokens.
L'énergie (J/1k tokens) est corrélée hors-ligne depuis DCGM (Prometheus),
PAS estimée ici — voir docs/energy-methodology.md.

Aucune valeur fabriquée : tant qu'on ne tourne pas contre un vrai endpoint,
ce script échoue proprement plutôt que d'inventer des chiffres.
"""
import argparse
import json
import os
import statistics
import time
from pathlib import Path

import requests  # requirements.txt


def one_request(endpoint: str, model: str, prompt: str, max_tokens: int) -> dict:
    t0 = time.perf_counter()
    r = requests.post(
        f"{endpoint}/v1/completions",
        json={"model": model, "prompt": prompt, "max_tokens": max_tokens, "stream": False},
        timeout=120,
    )
    r.raise_for_status()
    dt = time.perf_counter() - t0
    usage = r.json().get("usage", {})
    return {"latency_s": dt, "completion_tokens": usage.get("completion_tokens", 0)}


def run(args) -> dict:
    prompt = "Explain continuous batching in LLM inference in three sentences."
    lats, toks = [], []
    for _ in range(args.n):
        m = one_request(args.endpoint, args.model, prompt, args.max_tokens)
        lats.append(m["latency_s"])
        toks.append(m["completion_tokens"])

    lats.sort()
    total_tokens = sum(toks)
    wall = sum(lats)
    p50 = statistics.median(lats)
    p99 = lats[min(len(lats) - 1, int(0.99 * len(lats)))]
    tps = total_tokens / wall if wall else 0.0

    # Coût : €/h GPU (arg) -> €/1M tokens. Grille Scaleway L4 à renseigner.
    eur_per_1m = (args.gpu_eur_hour / 3600) * (wall / total_tokens) * 1_000_000 if total_tokens else 0.0

    result = {
        "run_id": args.run_id,
        "model": args.model,
        "n_requests": args.n,
        "p50_latency_s": round(p50, 4),
        "p99_latency_s": round(p99, 4),
        "tokens_per_s": round(tps, 2),
        "eur_per_1m_tokens": round(eur_per_1m, 4),
        "gpu_eur_hour": args.gpu_eur_hour,
    }
    return result


def persist(result: dict):
    # Écriture MariaDB si DSN présent, sinon dump JSON local (jamais silencieux).
    outdir = Path(__file__).parent / "results"
    outdir.mkdir(exist_ok=True)
    fp = outdir / f"{result['run_id']}.json"
    fp.write_text(json.dumps(result, indent=2))
    print(f"[bench] résultat écrit -> {fp}")

    dsn = os.environ.get("MARIADB_DSN")
    if not dsn:
        print("[bench] MARIADB_DSN absent -> pas d'insertion BDD (attendu en dev local)")
        return
    import pymysql  # optionnel, seulement si DSN fourni
    # dsn attendu : host:port:user:pass:db
    host, port, user, pw, db = dsn.split(":")
    conn = pymysql.connect(host=host, port=int(port), user=user, password=pw, database=db)
    with conn.cursor() as cur:
        cur.execute(
            """INSERT INTO bench_runs
               (run_id, model, n_requests, p50_latency_s, p99_latency_s,
                tokens_per_s, eur_per_1m_tokens, gpu_eur_hour)
               VALUES (%(run_id)s,%(model)s,%(n_requests)s,%(p50_latency_s)s,
                       %(p99_latency_s)s,%(tokens_per_s)s,%(eur_per_1m_tokens)s,%(gpu_eur_hour)s)""",
            result,
        )
    conn.commit()
    print("[bench] inséré dans MariaDB.bench_runs")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--endpoint", required=True, help="http://<vllm-svc>:8000")
    ap.add_argument("--model", default="mistral-7b")
    ap.add_argument("--run-id", required=True)
    ap.add_argument("--n", type=int, default=50)
    ap.add_argument("--max-tokens", type=int, default=128)
    ap.add_argument("--gpu-eur-hour", type=float, default=0.0,
                    help="Prix horaire GPU (grille Scaleway) pour le calcul €/1M tokens")
    args = ap.parse_args()

    result = run(args)
    print(json.dumps(result, indent=2))
    persist(result)


if __name__ == "__main__":
    main()
