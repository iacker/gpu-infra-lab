// cost-exporter : dérive €/token et J/1k tokens à partir de la puissance GPU RÉELLE (DCGM)
// et du débit tokens (vLLM), et les expose en métriques Prometheus.
//
// Différenciateur du lab : personne ne mesure le coût marginal réel d'un token.
// Source énergie = DCGM_FI_DEV_POWER_USAGE (watts carte), JAMAIS une estimation.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	eurPerToken = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "lab_eur_per_token",
		Help: "Coût marginal estimé par token (€), dérivé de la puissance GPU réelle et du prix horaire.",
	})
	joulesPer1kTokens = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "lab_joules_per_1k_tokens",
		Help: "Énergie par 1000 tokens (J), depuis DCGM_FI_DEV_POWER_USAGE. Mesure, pas estimation.",
	})
	gpuWatts = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "lab_gpu_power_watts",
		Help: "Puissance GPU instantanée (W), miroir de DCGM pour traçabilité.",
	})
)

func init() {
	prometheus.MustRegister(eurPerToken, joulesPer1kTokens, gpuWatts)
}

// collect : à câbler sur le vrai cluster.
//   - watts  <- query Prometheus: DCGM_FI_DEV_POWER_USAGE
//   - tps    <- query Prometheus: rate(vllm:generation_tokens_total[1m])
//
// Tant que promURL est vide, on n'émet RIEN (pas de valeur fabriquée).
func collect(promURL string, eurPerHour float64) {
	if promURL == "" {
		log.Println("[cost-exporter] --prometheus-url absent : aucune métrique émise (attendu hors cluster)")
		return
	}
	watts := queryScalar(promURL, "avg(DCGM_FI_DEV_POWER_USAGE)")
	tps := queryScalar(promURL, "sum(rate(vllm:generation_tokens_total[1m]))")
	if tps <= 0 {
		return // pas de débit -> pas de coût/token valide
	}
	eurPerHourNow := eurPerHour
	// €/token = (€/h) / (tokens/h) = (€/h) / (tps*3600)
	eurPerToken.Set(eurPerHourNow / (tps * 3600))
	// J/1k tokens = watts (J/s) / tps (tokens/s) * 1000
	joulesPer1kTokens.Set((watts / tps) * 1000)
	gpuWatts.Set(watts)
}

func main() {
	promURL := flag.String("prometheus-url", "", "Base URL Prometheus (ex http://prometheus:9090)")
	eurPerHour := flag.Float64("gpu-eur-hour", 0.0, "Prix horaire GPU (grille Scaleway)")
	addr := flag.String("addr", ":9105", "Adresse d'écoute")
	interval := flag.Duration("interval", 15*time.Second, "Période de collecte")
	flag.Parse()

	go func() {
		for {
			collect(*promURL, *eurPerHour)
			time.Sleep(*interval)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("[cost-exporter] /metrics sur %s (prom=%q, €/h=%.4f)", *addr, *promURL, *eurPerHour)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
