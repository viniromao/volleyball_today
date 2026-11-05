package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const (
	// chance de chuva (%) a partir da qual o bot avisa com 4h, 3h e 2h de antecedência
	limiarChuva = 60
	// o aviso com a previsão sai quando falta 1h; os de alerta, de 4h até 2h
	maxAntecedenciaClima = 4
	validadeCache        = 10 * time.Minute

	// Presidente Altino, Osasco/SP; dá pra trocar com VOLEI_LAT, VOLEI_LON e VOLEI_LOCAL
	latPadrao = "-23.5262"
	lonPadrao = "-46.7686"
)

var nomeLocal = envOu("VOLEI_LOCAL", "Presidente Altino")

type Previsao struct {
	Chance int     // probabilidade de chuva, em %
	Mm     float64 // chuva esperada na hora, em mm
	Temp   float64 // °C
}

type cachePrevisao struct {
	em   time.Time
	prev Previsao
}

type respostaOpenMeteo struct {
	Hourly struct {
		Time   []string  `json:"time"`
		Chance []float64 `json:"precipitation_probability"`
		Mm     []float64 `json:"precipitation"`
		Temp   []float64 `json:"temperature_2m"`
	} `json:"hourly"`
}

// chaveHora é como o Open-Meteo escreve cada hora: "2026-11-05T19:00".
func chaveHora(t time.Time, fuso *time.Location) string {
	return t.In(fuso).Format("2006-01-02T15:00")
}

func lerPrevisao(corpo []byte, chave string) (Previsao, error) {
	var r respostaOpenMeteo
	if err := json.Unmarshal(corpo, &r); err != nil {
		return Previsao{}, err
	}
	for i, t := range r.Hourly.Time {
		if t != chave {
			continue
		}
		if i >= len(r.Hourly.Chance) || i >= len(r.Hourly.Mm) || i >= len(r.Hourly.Temp) {
			break
		}
		return Previsao{
			Chance: int(r.Hourly.Chance[i] + 0.5),
			Mm:     r.Hourly.Mm[i],
			Temp:   r.Hourly.Temp[i],
		}, nil
	}
	return Previsao{}, fmt.Errorf("sem previsão pra %s", chave)
}

func (b *Bot) buscarPrevisao(ctx context.Context, quando time.Time) (Previsao, error) {
	chave := chaveHora(quando, b.fuso)
	if c, ok := b.cacheClima[chave]; ok && time.Since(c.em) < validadeCache {
		return c.prev, nil
	}

	q := url.Values{}
	q.Set("latitude", b.lat)
	q.Set("longitude", b.lon)
	q.Set("hourly", "precipitation_probability,precipitation,temperature_2m")
	q.Set("timezone", b.fuso.String())
	q.Set("forecast_days", "3")

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.open-meteo.com/v1/forecast?"+q.Encode(), nil)
	if err != nil {
		return Previsao{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Previsao{}, err
	}
	defer resp.Body.Close()
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Previsao{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Previsao{}, fmt.Errorf("open-meteo respondeu %s", resp.Status)
	}
	prev, err := lerPrevisao(corpo, chave)
	if err != nil {
		return Previsao{}, err
	}
	if b.cacheClima == nil {
		b.cacheClima = map[string]cachePrevisao{}
	}
	b.cacheClima[chave] = cachePrevisao{em: time.Now(), prev: prev}
	return prev, nil
}

// JanelaClima diz em qual janela de aviso o evento está: k = quantas horas
// (arredondando pra cima) faltam. Só vale de 1 a 4; fora disso devolve 0.
func JanelaClima(agora, inicio time.Time) int {
	falta := inicio.Sub(agora)
	if falta <= 0 {
		return 0
	}
	k := int((falta + time.Hour - 1) / time.Hour)
	if k > maxAntecedenciaClima {
		return 0
	}
	return k
}

func virgula(f float64) string {
	return strings.Replace(fmt.Sprintf("%.1f", f), ".", ",", 1)
}

// MensagemClima monta o aviso. k == 1 é a previsão de 1h antes (sempre sai);
// k de 2 a 4 é o alerta de chuva forte, que só sai se a chance for grande.
func (v *Votacao) MensagemClima(p Previsao, k int) string {
	if k > 1 {
		return fmt.Sprintf("☔ Atenção: a previsão de *%s* pro *%s* às %s (daqui a %dh) tem *%d%%* de chance de chuva (%s mm). Vale ficar de olho!",
			nomeLocal, v.Evento(), v.horario(), k, p.Chance, virgula(p.Mm))
	}
	var situacao string
	switch {
	case p.Chance >= limiarChuva:
		situacao = "☔ Chuva forte no radar"
	case p.Chance >= 30:
		situacao = "🌦️ Pode chover"
	default:
		situacao = "☀️ Tempo firme"
	}
	return fmt.Sprintf("%s! Previsão de *%s* pro *%s* às %s: *%d%%* de chance de chuva (%s mm), %.0f°C.",
		situacao, nomeLocal, v.Evento(), v.horario(), p.Chance, virgula(p.Mm), p.Temp)
}

// avisarClima confere de tempos em tempos se falta 1h (previsão) ou 2h a 4h
// (alerta de chuva) pro início de algum evento com horário marcado.
func (b *Bot) avisarClima() {
	for {
		b.climaAgora()
		time.Sleep(30 * time.Second)
	}
}

type avisoClima struct {
	chat   string
	v      *Votacao
	inicio time.Time
	k      int
}

func (b *Bot) climaAgora() {
	if !b.cli.IsConnected() {
		return
	}
	agora := time.Now()
	b.mu.Lock()
	var avisos []avisoClima
	for chat, g := range b.grupos {
		if len(g.Votacoes) == 0 {
			continue
		}
		v := g.Votacoes[0]
		inicio, ok := v.Inicio(b.fuso)
		if !ok {
			continue
		}
		if k := JanelaClima(agora, inicio); k > 0 && v.ChuvaAvisos&(1<<k) == 0 {
			avisos = append(avisos, avisoClima{chat, v, inicio, k})
		}
	}
	b.mu.Unlock()

	ctx := context.Background()
	for _, a := range avisos {
		jid, err := types.ParseJID(a.chat)
		if err != nil {
			continue
		}
		prev, err := b.buscarPrevisao(ctx, a.inicio)
		if err != nil {
			fmt.Fprintln(os.Stderr, "erro ao buscar a previsão do tempo:", err)
			continue
		}

		b.mu.Lock()
		g := b.grupos[a.chat]
		if len(g.Votacoes) == 0 || g.Votacoes[0] != a.v || a.v.ChuvaAvisos&(1<<a.k) != 0 {
			b.mu.Unlock()
			continue
		}
		a.v.ChuvaAvisos |= 1 << a.k
		var texto string
		if a.k == 1 || prev.Chance >= limiarChuva {
			texto = a.v.MensagemClima(prev, a.k)
		}
		if err := b.Salvar(); err != nil {
			fmt.Fprintln(os.Stderr, "erro ao salvar:", err)
		}
		b.mu.Unlock()

		if texto == "" {
			continue
		}
		_, err = b.cli.SendMessage(ctx, jid, &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String(texto)},
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "erro ao mandar aviso de clima:", err)
		}
	}
}

// climaSobDemanda responde ao !clima com a previsão pro horário do evento
// atual, sem esperar a janela de 1h e sem gastar os avisos automáticos.
func (b *Bot) climaSobDemanda(chat types.JID) {
	b.mu.Lock()
	var v *Votacao
	if g, ok := b.grupos[chat.String()]; ok && len(g.Votacoes) > 0 {
		v = g.Votacoes[0]
	}
	var inicio time.Time
	var ok bool
	if v != nil {
		inicio, ok = v.Inicio(b.fuso)
	}
	b.mu.Unlock()

	if !ok {
		b.responder(chat, "Sem horário marcado não tem previsão. Manda `!volei 19h` pra marcar. 🏐")
		return
	}
	if !time.Now().Before(inicio) {
		b.responder(chat, "O horário marcado já passou. 🏐")
		return
	}
	prev, err := b.buscarPrevisao(context.Background(), inicio)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao buscar a previsão do tempo:", err)
		b.responder(chat, "⚠️ Não consegui buscar a previsão agora, tenta de novo daqui a pouco.")
		return
	}
	b.responder(chat, v.MensagemClima(prev, 1))
}
