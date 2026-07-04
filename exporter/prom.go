// Client Prometheus HTTP minimal : query instantanée -> scalaire.
package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type promResponse struct {
	Data struct {
		Result []struct {
			Value [2]interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// queryScalar exécute une query instantanée et renvoie la 1re valeur, ou 0.
func queryScalar(base, query string) float64 {
	u := base + "/api/v1/query?query=" + url.QueryEscape(query)
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var pr promResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return 0
	}
	if len(pr.Data.Result) == 0 {
		return 0
	}
	s, ok := pr.Data.Result[0].Value[1].(string)
	if !ok {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}
