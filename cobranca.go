package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	pago       = "💰"
	faltaPagar = "💸"
)

// Pagante é quem já pagou. Quem não está na lista de pagantes está com 💸.
type Pagante struct {
	IDs  []string `json:"ids"`
	Nome string   `json:"nome"`
}

// Cobranca é a lista de pagamento do grupo, separada da lista do vôlei: tem
// seu próprio título, data, valor, Pix, elenco (add/remove) e mensagens. Não
// some na virada do dia; fica até alguém mandar `!temquepagar fechar`.
type Cobranca struct {
	Nome       string    `json:"nome,omitempty"`
	Dia        string    `json:"dia,omitempty"`
	Data       string    `json:"data,omitempty"`
	Valor      string    `json:"valor,omitempty"`
	Pix        string    `json:"pix,omitempty"`
	Favorecido string    `json:"favorecido,omitempty"`
	Pagantes   []Pagante `json:"pagantes,omitempty"`
	Elenco
	Historico
}

// Configurar lê o que vem depois de `!temquepagar`: título, data, valor, chave
// Pix e nome de quem recebe, nessa ordem e tudo opcional; o que não vier fica
// como estava. Ex.: `churrasco 17/09 82,20 14357877733 "Vinicius Romao"`.
func (c *Cobranca) Configurar(args string, hoje time.Time) string {
	var campos []string
	for _, t := range strings.Fields(args) {
		if n := len(campos); n > 0 && strings.EqualFold(campos[n-1], "r$") {
			campos[n-1] = "R$" + t
			continue
		}
		campos = append(campos, t)
	}

	var titulo []string
	i := 0
	for ; i < len(campos); i++ {
		t := campos[i]
		if _, ok := lerValor(t); ok || pareceChavePix(t) || reData.MatchString(t) {
			break
		}
		if strings.ContainsAny(t, "0123456789") {
			return fmt.Sprintf("Não entendi *%s*. Manda assim: `!temquepagar 82,20` (valor), ou `!temquepagar churrasco 17/09 82,20 chave \"Nome\"`.", t)
		}
		titulo = append(titulo, t)
	}
	nome := strings.TrimFunc(normaliza(strings.Join(titulo, " ")), ehAspas)

	var dia, data string
	if i < len(campos) && reData.MatchString(campos[i]) {
		d, erro, _ := LerData(campos[i], hoje)
		if erro != "" {
			return erro
		}
		dia, data = d.Format(formatoDia), d.Format(formatoData)
		i++
	}
	var valor string
	if i < len(campos) {
		if centavos, ok := lerValor(campos[i]); ok {
			valor = fmt.Sprintf("%d,%02d", centavos/100, centavos%100)
			i++
		}
	}

	if nome != "" {
		c.Nome = nome
	}
	if data != "" {
		c.Dia, c.Data = dia, data
	}
	if valor != "" {
		c.Valor = valor
	}
	if i < len(campos) {
		c.Pix = campos[i]
		c.Favorecido = strings.TrimFunc(strings.Join(campos[i+1:], " "), ehAspas)
	}
	return ""
}

// pareceChavePix separa uma chave Pix (CPF, telefone, e-mail, aleatória) de
// um valor digitado errado, pra `!temquepagar 82,2x` ainda dar erro.
func pareceChavePix(s string) bool {
	if strings.Contains(s, "@") || strings.Count(s, "-") >= 4 {
		return true
	}
	digitos := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			digitos++
		}
	}
	return digitos >= 8
}

var reValor = regexp.MustCompile(`(?i)^(?:r\$)?\s*(\d{1,6})(?:[.,](\d{1,2}))?\s*(?:r\$)?$`)

func lerValor(s string) (int, bool) {
	m := reValor.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	reais, _ := strconv.Atoi(m[1])
	centavos := 0
	if m[2] != "" {
		centavos, _ = strconv.Atoi(m[2])
		if len(m[2]) == 1 {
			centavos *= 10
		}
	}
	return reais*100 + centavos, true
}

func reais(centavos int) string {
	return fmt.Sprintf("R$ %d,%02d", centavos/100, centavos%100)
}

func (c *Cobranca) indice(ids []string) int {
	for i := range c.Pagantes {
		if temAlgum(c.Pagantes[i].IDs, ids) {
			return i
		}
	}
	return -1
}

// Pagar marca que a pessoa pagou.
func (c *Cobranca) Pagar(ids []string, nome string) {
	i := c.indice(ids)
	if i < 0 {
		c.Pagantes = append(c.Pagantes, Pagante{IDs: ids, Nome: nome})
		return
	}
	p := &c.Pagantes[i]
	for _, id := range ids {
		if !contem(p.IDs, id) {
			p.IDs = append(p.IDs, id)
		}
	}
	if nome != "" {
		p.Nome = nome
	}
}

// Despagar desfaz um !paguei: a pessoa volta pra 💸.
func (c *Cobranca) Despagar(ids []string) {
	if i := c.indice(ids); i >= 0 {
		c.Pagantes = append(c.Pagantes[:i], c.Pagantes[i+1:]...)
	}
}

// Desmarcar é o que o remove faz ao tirar alguém da lista.
func (c *Cobranca) Desmarcar(ids []string) { c.Despagar(ids) }

// Zerar desmarca todos os pagamentos, mantendo título, valor e Pix.
func (c *Cobranca) Zerar() { c.Pagantes = nil }

func (c *Cobranca) linhas(membros []Membro) []linha {
	var out []linha
	vistos := map[int]bool{}
	for _, m := range membros {
		if m.Proprio {
			continue
		}
		i := c.indice(m.IDs)
		if i < 0 {
			out = append(out, linha{ids: m.IDs, nome: m.Nome, marca: faltaPagar})
			continue
		}
		vistos[i] = true
		nome := c.Pagantes[i].Nome
		if nome == "" {
			nome = m.Nome
		}
		out = append(out, linha{ids: m.IDs, nome: nome, marca: pago})
	}
	// quem pagou mas não aparece mais entre os membros continua na lista
	for i, p := range c.Pagantes {
		if !vistos[i] {
			out = append(out, linha{ids: p.IDs, nome: p.Nome, marca: pago})
		}
	}
	ordena(out)
	return out
}

func (c *Cobranca) Chamada() string {
	return "Lista de pagamento atualizada"
}

func (c *Cobranca) Render(membros []Membro, hoje time.Time) string {
	linhas := c.linhas(membros)

	titulo := "Pagamento"
	if c.Nome != "" {
		titulo += " — " + c.Nome
	}
	if c.Dia != "" {
		quando := "Dia " + c.Dia
		if c.Data == hoje.Format(formatoData) {
			quando = "Hoje " + quando
		}
		titulo += " — " + quando
	}

	var b strings.Builder
	fmt.Fprintf(&b, "💰 *%s*\n", titulo)
	if c.Valor != "" {
		fmt.Fprintf(&b, "Valor por pessoa %s R$\n", c.Valor)
	}
	if c.Pix != "" {
		fmt.Fprintf(&b, "Pix %s", c.Pix)
		if c.Favorecido != "" {
			fmt.Fprintf(&b, " (%s)", c.Favorecido)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	conta := map[string]int{}
	for _, ln := range linhas {
		fmt.Fprintf(&b, "%s %s\n", ln.marca, ln.nome)
		conta[ln.marca]++
	}
	fmt.Fprintf(&b, "\n📊 *Resumo*\n💰 Pagaram: %d\n💸 Faltam pagar: %d\n", conta[pago], conta[faltaPagar])
	if centavos, ok := lerValor(c.Valor); ok && c.Valor != "" {
		fmt.Fprintf(&b, "💵 Arrecadado: %s de %s\n", reais(centavos*conta[pago]), reais(centavos*len(linhas)))
	}
	b.WriteString("\nQuem já pagou manda `!paguei` (ou `!paguei Fulano` pra marcar outra pessoa).")
	return b.String()
}
