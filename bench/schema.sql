-- Registre d'expériences de bench. Une ligne = un run.
CREATE TABLE IF NOT EXISTS bench_runs (
    id                 BIGINT AUTO_INCREMENT PRIMARY KEY,
    run_id             VARCHAR(64) NOT NULL UNIQUE,
    model              VARCHAR(128) NOT NULL,
    n_requests         INT NOT NULL,
    p50_latency_s      DECIMAL(10,4) NOT NULL,
    p99_latency_s      DECIMAL(10,4) NOT NULL,
    tokens_per_s       DECIMAL(10,2) NOT NULL,
    eur_per_1m_tokens  DECIMAL(12,4) NOT NULL,
    gpu_eur_hour       DECIMAL(8,4) NOT NULL,
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
