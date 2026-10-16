package main

import (
	"fmt"
	"strings"
	"time"
)

const validadePergunta = 5 * time.Minute

const cancelado = "Ok, cancelei. Não mudei nada. 👍"

type etapa int

const (
	perguntaFormato etapa = iota
	perguntaCriar
)

// Pendente é uma pergunta que o bot fez e está esperando a resposta
// (um número) de quem mandou o comando. Fica só na memória.
type Pendente struct {
	chat     string
	ids      []string
	cmd      string
	palavras string
	etapa    etapa
	nomes    []string
	faltam   []string
	criar    []string
	expira   time.Time
}

// Lista é a lista do grupo sobre a qual um comando com nomes roda.
type Lista struct {
	g       *Grupo
	v       *Votacao
	todos   []Membro
	membros []Membro
}

func comNome(cmd string) bool {
	switch cmd {
	case "!add", "!remove", "!eu", "!nao", "!não", "!talvez", "!paguei", "!naopaguei", "!nãopaguei":
		return true
	}
	return false
}

// podeCriar diz se o comando oferece adicionar quem não está na lista.
func podeCriar(cmd string) bool {
	switch cmd {
	case "!eu", "!nao", "!não", "!talvez", "!paguei":
		return true
	}
	return false
}

func pedirNome(cmd string) string {
	return fmt.Sprintf("Manda o nome: `%s Fulano`, ou `%s \"Jose Maria\"` pra nome composto.", cmd, cmd)
}

func perguntaFormatoTexto(palavras string) string {
	var cada []string
	for _, n := range nomesDe(palavras) {
		cada = append(cada, "*"+n+"*")
	}
	return fmt.Sprintf("*%s* é uma pessoa só ou mais de uma? Responde com o número:\n1. Um nome só: *%s*\n2. Vários nomes: %s\n3. Cancelar",
		normaliza(palavras), normaliza(palavras), strings.Join(cada, ", "))
}

func perguntaCriarTexto(nome string) string {
	return fmt.Sprintf("*%s* não existe na lista. Quer adicionar essa pessoa? Responde com o número:\n1. Sim, adicionar\n2. Cancelar (não faz nada)", nome)
}

// pendenteDe acha a pergunta em aberto de quem mandou a mensagem. Chamar com
// b.mu travado.
func (b *Bot) pendenteDe(chat string, ids []string) *Pendente {
	agora := time.Now()
	var vivos []*Pendente
	var achado *Pendente
	for _, p := range b.pendentes {
		if agora.After(p.expira) {
			continue
		}
		vivos = append(vivos, p)
		if p.chat == chat && temAlgum(p.ids, ids) {
			achado = p
		}
	}
	b.pendentes = vivos
	return achado
}

func (b *Bot) esquecer(p *Pendente) {
	for i, x := range b.pendentes {
		if x == p {
			b.pendentes = append(b.pendentes[:i], b.pendentes[i+1:]...)
			return
		}
	}
}

// guardar deixa p como a única pergunta em aberto de quem a recebeu.
func (b *Bot) guardar(p *Pendente) {
	if antiga := b.pendenteDe(p.chat, p.ids); antiga != nil {
		b.esquecer(antiga)
	}
	p.expira = time.Now().Add(validadePergunta)
	b.pendentes = append(b.pendentes, p)
}

// iniciar começa um comando com nome: se o texto tem várias palavras sem
// aspas, primeiro pergunta se é uma pessoa só ou mais.
func (b *Bot) iniciar(chat string, ids []string, cmd, resto string, l *Lista) (string, bool) {
	nomes := nomesDe(resto)
	if len(nomes) == 0 {
		return pedirNome(cmd), false
	}
	p := &Pendente{chat: chat, ids: ids, cmd: cmd, palavras: resto}
	if len(nomes) > 1 && !strings.ContainsFunc(resto, ehAspas) {
		p.etapa = perguntaFormato
		b.guardar(p)
		return perguntaFormatoTexto(resto), false
	}
	return b.seguir(p, l, nomes)
}

// seguir roda o comando com os nomes já separados, ou pergunta se quem não
// está na lista deve ser adicionado.
func (b *Bot) seguir(p *Pendente, l *Lista, nomes []string) (string, bool) {
	p.nomes = nomes
	if podeCriar(p.cmd) {
		faltam, erros := l.faltantes(nomes)
		if len(erros) > 0 {
			return strings.Join(erros, "\n"), false
		}
		if len(faltam) > 0 {
			p.etapa, p.faltam, p.criar = perguntaCriar, faltam, nil
			b.guardar(p)
			return perguntaCriarTexto(faltam[0]), false
		}
	}
	return l.executar(p.cmd, nomes, nil)
}

// responderPergunta trata o número que a pessoa mandou.
func (b *Bot) responderPergunta(p *Pendente, l *Lista, n int) (string, bool) {
	switch p.etapa {
	case perguntaFormato:
		switch n {
		case 1:
			b.esquecer(p)
			return b.seguir(p, l, []string{normaliza(p.palavras)})
		case 2:
			b.esquecer(p)
			return b.seguir(p, l, nomesDe(p.palavras))
		case 3:
			b.esquecer(p)
			return cancelado, false
		}
		return "Responde com 1, 2 ou 3.", false
	case perguntaCriar:
		switch n {
		case 1:
			p.criar = append(p.criar, p.faltam[0])
			p.faltam = p.faltam[1:]
			if len(p.faltam) > 0 {
				p.expira = time.Now().Add(validadePergunta)
				return perguntaCriarTexto(p.faltam[0]), false
			}
			b.esquecer(p)
			return l.executar(p.cmd, p.nomes, p.criar)
		case 2:
			b.esquecer(p)
			return cancelado, false
		}
		return "Responde com 1 ou 2.", false
	}
	return "", false
}

// faltantes separa os nomes que ninguém na lista tem (dá pra oferecer
// adicionar) dos que deram erro, como nome repetido.
func (l *Lista) faltantes(nomes []string) (faltam, erros []string) {
	for _, n := range nomes {
		_, erro, achou := buscar(l.v.linhas(l.membros), n)
		switch {
		case erro == "":
		case !achou:
			if !contem(faltam, n) {
				faltam = append(faltam, n)
			}
		default:
			erros = append(erros, erro)
		}
	}
	return faltam, erros
}

// executar aplica cmd a cada nome; os de criar entram na lista antes. O bool
// diz se a lista mudou e deve ser mostrada de novo.
func (l *Lista) executar(cmd string, nomes, criar []string) (string, bool) {
	var erros []string
	for _, n := range nomes {
		var erro string
		switch cmd {
		case "!add":
			erro = l.g.Incluir(l.membros, n)
		case "!remove":
			erro = l.g.Tirar(l.v, l.membros, n)
		default:
			erro = l.marcar(cmd, n, criar)
		}
		if erro != "" {
			erros = append(erros, erro)
		}
		l.membros = l.g.Ajustar(l.todos)
	}
	return strings.Join(erros, "\n"), len(erros) < len(nomes)
}

func (l *Lista) marcar(cmd, nome string, criar []string) string {
	if contem(criar, nome) {
		if _, _, achou := buscar(l.v.linhas(l.membros), nome); !achou {
			if erro := l.g.Incluir(l.membros, nome); erro != "" {
				return erro
			}
			l.membros = l.g.Ajustar(l.todos)
		}
	}
	ids, erro := l.v.Encontrar(l.membros, nome)
	if erro != "" {
		return erro
	}
	switch cmd {
	case "!paguei":
		l.v.Pagar(ids, "")
	case "!naopaguei", "!nãopaguei":
		l.v.Despagar(ids)
	case "!nao", "!não":
		l.v.Marcar(ids, "", NaoVai)
	case "!talvez":
		l.v.Marcar(ids, "", Talvez)
	default:
		l.v.Marcar(ids, "", Vai)
	}
	return ""
}
