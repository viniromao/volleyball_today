package main

import (
	"testing"
	"time"
)

func TestJanelaClima(t *testing.T) {
	inicio := time.Date(2026, 11, 5, 19, 0, 0, 0, time.UTC)
	casos := []struct {
		antes time.Duration
		want  int
	}{
		{5 * time.Hour, 0},
		{4*time.Hour + time.Minute, 0},
		{4 * time.Hour, 4},
		{3*time.Hour + 30*time.Minute, 4},
		{3 * time.Hour, 3},
		{2 * time.Hour, 2},
		{90 * time.Minute, 2},
		{time.Hour, 1},
		{time.Minute, 1},
		{0, 0},
		{-time.Minute, 0},
	}
	for _, c := range casos {
		if got := JanelaClima(inicio.Add(-c.antes), inicio); got != c.want {
			t.Errorf("faltando %v: janela %d, esperava %d", c.antes, got, c.want)
		}
	}
}

func TestLerPrevisao(t *testing.T) {
	corpo := []byte(`{"hourly":{"time":["2026-11-05T18:00","2026-11-05T19:00"],
		"precipitation_probability":[10,72],"precipitation":[0,1.24],"temperature_2m":[23.1,21.6]}}`)
	p, err := lerPrevisao(corpo, "2026-11-05T19:00")
	if err != nil || p.Chance != 72 || p.Mm != 1.24 || p.Temp != 21.6 {
		t.Fatalf("got %+v, %v", p, err)
	}
	if _, err := lerPrevisao(corpo, "2026-11-06T19:00"); err == nil {
		t.Fatal("esperava erro pra hora sem previsão")
	}
}

func TestMensagemClima(t *testing.T) {
	v := &Votacao{Hora: "19:00"}
	if m := v.MensagemClima(Previsao{Chance: 80, Mm: 2.5, Temp: 20}, 3); m == "" {
		t.Fatal("mensagem vazia")
	}
	t.Log(v.MensagemClima(Previsao{Chance: 80, Mm: 2.5, Temp: 20}, 1))
}
