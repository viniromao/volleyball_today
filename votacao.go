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

const mensagemAjuda = "🏐 *Comandos do bot do Volei*\n\n" +
	"*Responder*\n" +
	"• `!eu` — vou (✅)\n" +
	"• `!nao` — não vou (❌)\n" +
	"• `!talvez` — tô na dúvida (🤔)\n" +
	"• `!eu Fulano`, `!nao Fulano`, `!talvez Fulano` — marca outra pessoa. Vale um pedaço do nome, sem ligar pra maiúscula nem acento. Se tiver mais de um com o nome, o bot mostra as opções numeradas e você manda `!eu jose 2`.\n" +
	"Dá pra trocar a resposta quantas vezes quiser, vale a última.\n\n" +
	"*Lista*\n" +
	"• `!volei` — mostra a lista (a de hoje, ou a do dia marcado)\n" +
	"• `!volei 17/09` — monta a lista pro dia 17/09. Ela vale até esse dia acabar; trocar de data começa do zero\n" +
	"• `!volei churrasco 17/09` — mesma coisa, com outro nome no título\n" +
	"• `!volei zerar` — limpa as respostas, mantendo dia e nome\n" +
	"• `!abortarmissao` — cancela o evento e volta pro vôlei de hoje, do zero\n\n" +
	"*Pagamento*\n" +
	"• `!temquepagar` — liga a cobrança na lista atual: quem vai ganha 💸 embaixo do nome\n" +
	"• `!temquepagar 82,20` — idem, mostrando o valor por pessoa embaixo do título\n" +
	"• `!temquepagar 82,20 fulano@email.com \"Fulano de Tal\"` — idem, com a chave Pix e o nome de quem recebe\n" +
	"• `!paguei` / `!paguei Fulano` — marca que pagou (💰); quem paga sem ter confirmado vira ✅\n" +
	"• `!naopaguei` / `!naopaguei Fulano` — desfaz o pagamento (volta pra 💸)\n" +
	"Dá pra ligar sem valor e mandar `!temquepagar 82,20` depois; mandar de novo troca o valor. Toda lista nova começa sem cobrança.\n\n" +
	"*Quem aparece na lista* (fica guardado pro grupo, vale todo dia)\n" +
	"• `!add Jose` — adiciona alguém que não está no grupo, ou põe de volta quem foi removido\n" +
	"• `!remove Jose` — tira alguém da lista de vez\n" +
	"• Nome composto vai entre aspas: `!add \"Jose Maria\"`. Sem aspas, cada palavra é uma pessoa: `!add Jose Joao` adiciona duas\n" +
	"• `!volei config` — mostra quem foi removido e quem foi adicionado\n\n" +
	"• `!volei ajuda` — esta mensagem"

type Confirmado struct {
	IDs    []string `json:"ids"`
	Nome   string   `json:"nome"`
	Nao    bool     `json:"nao,omitempty"`
	Talvez bool     `json:"talvez,omitempty"`
	Pago   bool     `json:"pago,omitempty"`
}

type Resposta int

const (
	Vai Resposta = iota
	NaoVai
	Talvez
)

type Mensagem struct {
	ID string    `json:"id"`
	Em time.Time `json:"em"`
}

type Votacao struct {
	Dia         string       `json:"dia"`
	Data        string       `json:"data,omitempty"`
	Nome        string       `json:"nome,omitempty"`
	Cobranca    bool         `json:"cobranca,omitempty"`
	Valor       string       `json:"valor,omitempty"`
	Pix         string       `json:"pix,omitempty"`
	Favorecido  string       `json:"favorecido,omitempty"`
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
	ln, erro, _ := buscar(v.linhas(membros), busca)
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
	if ln, erro, achou := buscar(ocultos, nome); erro == "" {
		for i, o := range g.Ocultos {
			if temAlgum(o.IDs, ln.ids) {
				g.Ocultos = append(g.Ocultos[:i], g.Ocultos[i+1:]...)
				break
			}
		}
		return ""
	} else if achou {
		return erro
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
// estiver entre aspas conta como um nome só. Um número solto vai junto com o
// nome de antes, pra escolher entre nomes iguais: jose 2.
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
			if _, err := strconv.Atoi(nome); err == nil && len(nomes) > 0 {
				nomes[len(nomes)-1] += " " + nome
				continue
			}
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

func (v *Votacao) Marcar(ids []string, nome string, r Resposta) {
	i := v.indice(ids)
	if i < 0 {
		v.Confirmados = append(v.Confirmados, Confirmado{IDs: ids, Nome: nome, Nao: r == NaoVai, Talvez: r == Talvez})
		return
	}
	c := &v.Confirmados[i]
	c.Nao, c.Talvez = r == NaoVai, r == Talvez
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
	ids       []string
	nome      string
	marca     string
	pagamento string
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
	if c.Talvez {
		return "🤔"
	}
	if c.Nao {
		return "❌"
	}
	return "✅"
}

const (
	pago       = "💰 pago"
	faltaPagar = "💸 falta pagar"
)

// pagamento é a linha que vai embaixo do nome quando a lista tem cobrança:
// quem vai e não pagou fica com 💸; quem pagou fica com 💰 mesmo se desistiu.
func (v *Votacao) pagamento(c Confirmado) string {
	switch {
	case !v.Cobranca:
		return ""
	case c.Pago:
		return pago
	case !c.Nao && !c.Talvez:
		return faltaPagar
	}
	return ""
}

// Cobrar liga a cobrança na votação atual. O que vier depois é opcional:
// valor por pessoa, chave Pix e nome de quem recebe, nessa ordem
// (`87,29 14357877733 "Vinicius Romao"`); o que não vier fica como estava.
func (v *Votacao) Cobrar(args string) string {
	campos := strings.Fields(args)
	if len(campos) > 1 && strings.EqualFold(campos[0], "r$") {
		campos = append([]string{"R$" + campos[1]}, campos[2:]...)
	}
	if len(campos) > 0 {
		if centavos, ok := lerValor(campos[0]); ok {
			v.Valor = fmt.Sprintf("%d,%02d", centavos/100, centavos%100)
			campos = campos[1:]
		} else if len(campos) == 1 && !pareceChavePix(campos[0]) {
			return fmt.Sprintf("Não entendi o valor *%s*. Manda assim: `!temquepagar 82,20`.", campos[0])
		}
	}
	if len(campos) > 0 {
		v.Pix = campos[0]
		v.Favorecido = strings.TrimFunc(strings.Join(campos[1:], " "), ehAspas)
	}
	v.Cobranca = true
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

// Pagar marca que a pessoa pagou; quem paga passa a constar como quem vai.
func (v *Votacao) Pagar(ids []string, nome string) {
	v.Marcar(ids, nome, Vai)
	v.Confirmados[v.indice(ids)].Pago = true
}

// Despagar desfaz um !paguei, sem mexer na resposta da pessoa.
func (v *Votacao) Despagar(ids []string) {
	if i := v.indice(ids); i >= 0 {
		v.Confirmados[i].Pago = false
	}
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
		out = append(out, linha{ids: m.IDs, nome: nome, marca: c.marca(), pagamento: v.pagamento(c)})
	}
	for i, c := range v.Confirmados {
		if !vistos[i] {
			out = append(out, linha{ids: c.IDs, nome: c.Nome, marca: c.marca(), pagamento: v.pagamento(c)})
		}
	}
	// Nome igual desempata pelos IDs, pra "jose 2" ser sempre a mesma pessoa.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := simplifica(out[i].nome), simplifica(out[j].nome)
		if a != b {
			return a < b
		}
		return strings.Join(out[i].ids, ",") < strings.Join(out[j].ids, ",")
	})
	return out
}

func (v *Votacao) Encontrar(membros []Membro, busca string) ([]string, string) {
	ln, erro, _ := buscar(v.linhas(membros), busca)
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

// buscar acha uma pessoa só. Quando mais de uma bate, a resposta numera as
// opções e dá pra escolher mandando o número no fim: "jose 2". O bool diz se
// alguém bateu com a busca, mesmo que não tenha dado pra escolher.
func buscar(linhas []linha, busca string) (linha, string, bool) {
	alvo := normaliza(busca)
	achados := casar(linhas, alvo)
	if len(achados) == 1 {
		return achados[0], "", true
	}
	if campos := strings.Fields(alvo); len(campos) > 1 {
		if n, err := strconv.Atoi(campos[len(campos)-1]); err == nil {
			nome := strings.Join(campos[:len(campos)-1], " ")
			if opcoes := casar(linhas, nome); len(opcoes) > 0 {
				if n >= 1 && n <= len(opcoes) {
					return opcoes[n-1], "", true
				}
				return linha{}, fmt.Sprintf("Não tem o número *%d* pra *%s*:\n%s", n, nome, numera(opcoes)), true
			}
		}
	}
	if len(achados) == 0 {
		return linha{}, fmt.Sprintf("Não achei ninguém com *%s* no nome.", alvo), false
	}
	return linha{}, fmt.Sprintf("Achei mais de uma pessoa com *%s*:\n%s\nManda mais do nome ou o número no fim, tipo *%s 2*.", alvo, numera(achados), alvo), true
}

func numera(linhas []linha) string {
	var b strings.Builder
	for i, ln := range linhas {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%d. %s", i+1, strings.TrimSpace(ln.marca+" "+ln.nome))
	}
	return b.String()
}

func (v *Votacao) Render(membros []Membro, hoje time.Time) string {
	linhas := v.linhas(membros)
	ehHoje := v.Data == hoje.Format(formatoData)

	var b strings.Builder
	quando := "Dia " + v.Dia
	if ehHoje {
		quando = "Hoje " + quando
	}
	fmt.Fprintf(&b, "🏐 *%s — %s*\n", v.Evento(), quando)
	if v.Cobranca && v.Valor != "" {
		fmt.Fprintf(&b, "Valor por pessoa %s R$\n", v.Valor)
	}
	if v.Cobranca && v.Pix != "" {
		fmt.Fprintf(&b, "Pix %s", v.Pix)
		if v.Favorecido != "" {
			fmt.Fprintf(&b, " (%s)", v.Favorecido)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	conta := map[string]int{}
	for _, ln := range linhas {
		fmt.Fprintf(&b, "%s %s\n", ln.marca, ln.nome)
		if ln.pagamento != "" {
			fmt.Fprintf(&b, "  %s\n", ln.pagamento)
		}
		conta[ln.marca]++
		conta[ln.pagamento]++
	}
	fmt.Fprintf(&b, "\n📊 *Resumo*\n✅ Confirmados: %d\n🤔 Talvez: %d\n❌ Não vão: %d\n▫️ Sem resposta: %d\n",
		conta["✅"], conta["🤔"], conta["❌"], conta["▫️"])
	if v.Cobranca {
		fmt.Fprintf(&b, "💰 Pagaram: %d\n💸 Faltam pagar: %d\n", conta[pago], conta[faltaPagar])
	}
	onde := ""
	switch {
	case v.Nome != "":
	case ehHoje:
		onde = " no vôlei de hoje"
	default:
		onde = " no vôlei do dia " + v.Dia
	}
	fmt.Fprintf(&b, "\nComente `!eu` pra confirmar presença%s, `!talvez` se estiver na dúvida ou `!nao` se não for. Dá pra trocar quantas vezes quiser.", onde)
	if v.Cobranca {
		b.WriteString(" Quem já pagou manda `!paguei`.")
	}
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
