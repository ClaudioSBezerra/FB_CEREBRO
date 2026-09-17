package main

// proxy_helper.go — GET genérico com header custom, usado pelos clients
// de FAROL e SmartPick pras rotas que só repassam a resposta (nunca
// recalculam, AD-3 da espinha) — extraído aqui porque os dois clients
// precisam do exato mesmo formato (monta request, seta 1 header, lê o
// corpo cru), não é abstração especulativa.

import (
	"io"
	"net/http"
)

func proxyGet(httpClient *http.Client, reqURL, headerNome, headerValor string) (status int, body []byte, err error) {
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set(headerNome, headerValor)

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}
