package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
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
	Confirmados []Confirmado `json:"confirmados"`
	Mensagens   []Mensagem   `json:"mensagens"`
	Envios      int          `json:"envios,omitempty"`
}

type Grupo struct {
	Votacoes []*Votacao `json:"votacoes"`
}

type Membro struct {
	IDs     []string
	Nome    string
	Proprio bool
}

func normaliza(nome string) string {
	return strings.Join(strings.Fields(nome), " ")
}

func (g *Grupo) Hoje(dia string) *Votacao {
	var v *Votacao
	for _, x := range g.Votacoes {
		if x.Dia == dia {
			v = x
			break
		}
	}
	if v == nil {
		v = &Votacao{Dia: dia}
	}
	g.Votacoes = []*Votacao{v}
	return v
}

func (v *Votacao) Zerar(dia string) {
	v.Confirmados = nil
	v.Dia = dia
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
	alvo := simplifica(busca)
	var exatos, parciais []linha
	for _, ln := range v.linhas(membros) {
		nome := simplifica(ln.nome)
		switch {
		case nome == alvo:
			exatos = append(exatos, ln)
		case strings.Contains(nome, alvo):
			parciais = append(parciais, ln)
		}
	}
	achados := exatos
	if len(achados) == 0 {
		achados = parciais
	}
	switch len(achados) {
	case 0:
		return nil, fmt.Sprintf("Não achei ninguém com *%s* no nome.", normaliza(busca))
	case 1:
		return achados[0].ids, ""
	}
	var nomes []string
	for _, ln := range achados {
		nomes = append(nomes, ln.nome)
	}
	return nil, fmt.Sprintf("Achei mais de uma pessoa com *%s*: %s. Manda mais do nome.", normaliza(busca), strings.Join(nomes, ", "))
}

func (v *Votacao) Render(membros []Membro) string {
	linhas := v.linhas(membros)

	var b strings.Builder
	fmt.Fprintf(&b, "🏐 *Volei — Hoje Dia %s*\n\n", v.Dia)
	for _, ln := range linhas {
		fmt.Fprintf(&b, "%s %s\n", ln.marca, ln.nome)
	}
	b.WriteString("\nComente `!eu` pra confirmar presença no vôlei de hoje ou `!nao` se não for. Dá pra trocar quantas vezes quiser.")
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
