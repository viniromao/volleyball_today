package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxMensagens = 30

const mensagemAbortada = "🚨 *MISSÃO ABORTADA* 🚨\n\n" +
	"Operação cancelada. Não pergunta, não reclama, não manda áudio de 4 minutos. Só aceita."

type Confirmado struct {
	IDs  []string `json:"ids"`
	Nome string   `json:"nome"`
	Nao  bool     `json:"nao,omitempty"`
}

type Mensagem struct {
	ID string    `json:"id"`
	Em time.Time `json:"em"`
}

type Votacao struct {
	Dia         string       `json:"dia"`
	Data        string       `json:"data,omitempty"`
	Nome        string       `json:"nome,omitempty"`
	Confirmados []Confirmado `json:"confirmados"`
	Mensagens   []Mensagem   `json:"mensagens"`
	Envios      int          `json:"envios,omitempty"`
}

type Pessoa struct {
	IDs  []string `json:"ids"`
	Nome string   `json:"nome"`
}

type Grupo struct {
	Votacoes []*Votacao `json:"votacoes"`
	Ocultos  []Pessoa   `json:"ocultos,omitempty"`
	Extras   []string   `json:"extras,omitempty"`
}

const prefixoExtra = "extra:"

func idExtra(nome string) string {
	return prefixoExtra + simplifica(nome)
}

// Ajustar tira da lista quem foi escondido com !remove e põe os nomes
// incluídos à mão com !add.
func (g *Grupo) Ajustar(membros []Membro) []Membro {
	var out []Membro
	for _, m := range membros {
		if !g.oculto(m.IDs) {
			out = append(out, m)
		}
	}
	for _, nome := range g.Extras {
		out = append(out, Membro{IDs: []string{idExtra(nome)}, Nome: nome})
	}
	return out
}

func (g *Grupo) oculto(ids []string) bool {
	for _, o := range g.Ocultos {
		if temAlgum(o.IDs, ids) {
			return true
		}
	}
	return false
}

func (g *Grupo) Tirar(v *Votacao, membros []Membro, busca string) string {
	ln, erro := buscar(v.linhas(membros), busca)
	if erro != "" {
		return erro
	}
	if id := ln.ids[0]; strings.HasPrefix(id, prefixoExtra) {
		for i, nome := range g.Extras {
			if idExtra(nome) == id {
				g.Extras = append(g.Extras[:i], g.Extras[i+1:]...)
				break
			}
		}
	} else {
		g.Ocultos = append(g.Ocultos, Pessoa{IDs: ln.ids, Nome: ln.nome})
	}
	v.Desmarcar(ln.ids)
	return ""
}

func (g *Grupo) Incluir(membros []Membro, nome string) string {
	nome = normaliza(nome)
	var ocultos []linha
	for _, o := range g.Ocultos {
		ocultos = append(ocultos, linha{ids: o.IDs, nome: o.Nome})
	}
	switch achados := casar(ocultos, nome); len(achados) {
	case 0:
	case 1:
		for i, o := range g.Ocultos {
			if temAlgum(o.IDs, achados[0].ids) {
				g.Ocultos = append(g.Ocultos[:i], g.Ocultos[i+1:]...)
				break
			}
		}
		return ""
	default:
		return fmt.Sprintf("Achei mais de uma pessoa tirada da lista com *%s*: %s. Manda mais do nome.", nome, juntaNomes(achados))
	}
	for _, m := range membros {
		if simplifica(m.Nome) == simplifica(nome) {
			return fmt.Sprintf("*%s* já está na lista.", m.Nome)
		}
	}
	g.Extras = append(g.Extras, nome)
	return ""
}

func (g *Grupo) Config() string {
	var b strings.Builder
	b.WriteString("⚙️ *Config do Volei neste grupo*\n\n")
	var ocultos []string
	for _, o := range g.Ocultos {
		ocultos = append(ocultos, o.Nome)
	}
	b.WriteString("Fora da lista: ")
	b.WriteString(ouNinguem(ocultos))
	b.WriteString("\nIncluídos à mão: ")
	b.WriteString(ouNinguem(g.Extras))
	b.WriteString("\n\n`!remove Fulano` tira alguém da lista, `!add Fulano` põe de volta ou adiciona quem não está no grupo. Nome composto vai entre aspas: `!add \"Jose Maria\"`.")
	return b.String()
}

func ehAspas(r rune) bool {
	return r == '"' || r == '“' || r == '”'
}

// nomesDe separa os nomes de um !add/!remove: cada palavra é um nome, e o que
// estiver entre aspas conta como um nome só.
func nomesDe(s string) []string {
	var nomes []string
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			return nomes
		}
		var nome string
		if r, tam := utf8.DecodeRuneInString(s); ehAspas(r) {
			s = s[tam:]
			i := strings.IndexFunc(s, ehAspas)
			if i < 0 {
				i = len(s)
			}
			nome = s[:i]
			s = strings.TrimLeftFunc(s[i:], ehAspas)
		} else {
			i := strings.IndexFunc(s, unicode.IsSpace)
			if i < 0 {
				i = len(s)
			}
			nome, s = s[:i], s[i:]
		}
		if nome = normaliza(nome); nome != "" {
			nomes = append(nomes, nome)
		}
	}
}

func ouNinguem(nomes []string) string {
	if len(nomes) == 0 {
		return "ninguém"
	}
	return strings.Join(nomes, ", ")
}

type Membro struct {
	IDs     []string
	Nome    string
	Proprio bool
}

func normaliza(nome string) string {
	return strings.Join(strings.Fields(nome), " ")
}

const (
	formatoDia  = "02/01"
	formatoData = "2006-01-02"
)

var reData = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})$`)

// LerData entende "17/09" como a próxima vez que esse dia chega, contando a
// partir de hoje: em dezembro, "03/01" é janeiro do ano que vem.
func LerData(s string, hoje time.Time) (time.Time, string, bool) {
	m := reData.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, "", false
	}
	dia, _ := strconv.Atoi(m[1])
	mes, _ := strconv.Atoi(m[2])
	for _, ano := range []int{hoje.Year(), hoje.Year() + 1} {
		d := time.Date(ano, time.Month(mes), dia, 0, 0, 0, 0, hoje.Location())
		if d.Day() != dia || int(d.Month()) != mes {
			continue
		}
		if !d.Before(hoje) && d.Sub(hoje) <= 183*24*time.Hour {
			return d, "", true
		}
	}
	return time.Time{}, fmt.Sprintf("Não dá pra marcar pro dia *%s*: essa data já passou ou não existe.", s), true
}

// Atual devolve a votação que ainda vale: a de hoje ou a de um dia marcado
// mais pra frente com !volei 17/09. Passou o dia, começa uma nova pra hoje.
func (g *Grupo) Atual(hoje time.Time) *Votacao {
	dia, data := hoje.Format(formatoDia), hoje.Format(formatoData)
	var v *Votacao
	for _, x := range g.Votacoes {
		if x.Data == "" && x.Dia == dia {
			x.Data = data
		}
		if x.Data >= data {
			v = x
			break
		}
	}
	if v == nil {
		v = &Votacao{Dia: dia, Data: data}
	}
	g.Votacoes = []*Votacao{v}
	return v
}

// Agendar troca a votação pra outro dia (começando do zero) ou só renomeia se
// o dia é o mesmo. As mensagens passam junto pra lista antiga apontar pra nova.
func (g *Grupo) Agendar(v *Votacao, nome string, dia time.Time) *Votacao {
	if data := dia.Format(formatoData); v.Data != data {
		v = &Votacao{Dia: dia.Format(formatoDia), Data: data, Mensagens: v.Mensagens, Envios: v.Envios}
		g.Votacoes = []*Votacao{v}
	}
	v.Nome = nome
	return v
}

func (v *Votacao) Evento() string {
	if v.Nome == "" {
		return "Volei"
	}
	return v.Nome
}

func (v *Votacao) Chamada() string {
	if v.Nome == "" {
		return "Lista atualizada do *Volei*"
	}
	return fmt.Sprintf("Lista atualizada (*%s*)", v.Nome)
}

func (v *Votacao) Zerar() {
	v.Confirmados = nil
}

func (v *Votacao) Registrar(id string, em, limite time.Time) ([]string, int) {
	var editaveis []string
	for _, m := range v.Mensagens {
		if m.Em.After(limite) {
			editaveis = append(editaveis, m.ID)
		}
	}
	if v.Envios < len(v.Mensagens) {
		v.Envios = len(v.Mensagens)
	}
	v.Envios++
	v.Mensagens = append(v.Mensagens, Mensagem{ID: id, Em: em})
	if len(v.Mensagens) > maxMensagens {
		v.Mensagens = v.Mensagens[len(v.Mensagens)-maxMensagens:]
	}
	return editaveis, v.Envios
}

func (v *Votacao) indice(ids []string) int {
	for i := range v.Confirmados {
		if temAlgum(v.Confirmados[i].IDs, ids) {
			return i
		}
	}
	return -1
}

func (v *Votacao) Desmarcar(ids []string) {
	if i := v.indice(ids); i >= 0 {
		v.Confirmados = append(v.Confirmados[:i], v.Confirmados[i+1:]...)
	}
}

func (v *Votacao) Marcar(ids []string, nome string, nao bool) {
	i := v.indice(ids)
	if i < 0 {
		v.Confirmados = append(v.Confirmados, Confirmado{IDs: ids, Nome: nome, Nao: nao})
		return
	}
	c := &v.Confirmados[i]
	c.Nao = nao
	for _, id := range ids {
		if !contem(c.IDs, id) {
			c.IDs = append(c.IDs, id)
		}
	}
	if nome != "" {
		c.Nome = nome
	}
}

type linha struct {
	ids   []string
	nome  string
	marca string
}

var semAcento = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

func simplifica(s string) string {
	return semAcento.Replace(strings.ToLower(normaliza(s)))
}

func (c Confirmado) marca() string {
	if c.Nao {
		return "❌"
	}
	return "✅"
}

func (v *Votacao) linhas(membros []Membro) []linha {
	var out []linha
	vistos := map[int]bool{}
	for _, m := range membros {
		i := v.indice(m.IDs)
		if i < 0 {
			if !m.Proprio {
				out = append(out, linha{ids: m.IDs, nome: m.Nome, marca: "▫️"})
			}
			continue
		}
		vistos[i] = true
		c := v.Confirmados[i]
		nome := c.Nome
		if nome == "" {
			nome = m.Nome
		}
		out = append(out, linha{ids: m.IDs, nome: nome, marca: c.marca()})
	}
	for i, c := range v.Confirmados {
		if !vistos[i] {
			out = append(out, linha{ids: c.IDs, nome: c.Nome, marca: c.marca()})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return simplifica(out[i].nome) < simplifica(out[j].nome)
	})
	return out
}

func (v *Votacao) Encontrar(membros []Membro, busca string) ([]string, string) {
	ln, erro := buscar(v.linhas(membros), busca)
	return ln.ids, erro
}

// casar devolve quem tem o nome igual à busca ou, se ninguém tiver, quem
// contém a busca no nome.
func casar(linhas []linha, busca string) []linha {
	alvo := simplifica(busca)
	var exatos, parciais []linha
	for _, ln := range linhas {
		nome := simplifica(ln.nome)
		switch {
		case nome == alvo:
			exatos = append(exatos, ln)
		case strings.Contains(nome, alvo):
			parciais = append(parciais, ln)
		}
	}
	if len(exatos) > 0 {
		return exatos
	}
	return parciais
}

func buscar(linhas []linha, busca string) (linha, string) {
	achados := casar(linhas, busca)
	switch len(achados) {
	case 0:
		return linha{}, fmt.Sprintf("Não achei ninguém com *%s* no nome.", normaliza(busca))
	case 1:
		return achados[0], ""
	}
	return linha{}, fmt.Sprintf("Achei mais de uma pessoa com *%s*: %s. Manda mais do nome.", normaliza(busca), juntaNomes(achados))
}

func juntaNomes(linhas []linha) string {
	var nomes []string
	for _, ln := range linhas {
		nomes = append(nomes, ln.nome)
	}
	return strings.Join(nomes, ", ")
}

func (v *Votacao) Render(membros []Membro, hoje time.Time) string {
	linhas := v.linhas(membros)
	ehHoje := v.Data == hoje.Format(formatoData)

	var b strings.Builder
	quando := "Dia " + v.Dia
	if ehHoje {
		quando = "Hoje " + quando
	}
	fmt.Fprintf(&b, "🏐 *%s — %s*\n\n", v.Evento(), quando)
	for _, ln := range linhas {
		fmt.Fprintf(&b, "%s %s\n", ln.marca, ln.nome)
	}
	onde := ""
	switch {
	case v.Nome != "":
	case ehHoje:
		onde = " no vôlei de hoje"
	default:
		onde = " no vôlei do dia " + v.Dia
	}
	fmt.Fprintf(&b, "\nComente `!eu` pra confirmar presença%s ou `!nao` se não for. Dá pra trocar quantas vezes quiser.", onde)
	return b.String()
}

func contem(lista []string, s string) bool {
	for _, x := range lista {
		if x == s {
			return true
		}
	}
	return false
}

func temAlgum(a, b []string) bool {
	for _, x := range a {
		if contem(b, x) {
			return true
		}
	}
	return false
}
