package main

import (
	"strings"
	"testing"
	"time"
)

func novaLista() (*Bot, *Lista) {
	todos := []Membro{{IDs: []string{"1"}, Nome: "Luana"}, {IDs: []string{"2"}, Nome: "Jose"}}
	g := &Grupo{}
	v := g.Atual(time.Now())
	return &Bot{}, &Lista{g: g, v: v, todos: todos, membros: g.Ajustar(todos)}
}

func TestVariasPalavrasPerguntaFormato(t *testing.T) {
	b, l := novaLista()
	aviso, publica := b.iniciar("g", []string{"9"}, "!eu", "irmao da luana", l)
	if publica || !strings.Contains(aviso, "1. Um nome só") {
		t.Fatalf("esperava pergunta de formato, veio %q", aviso)
	}
	aviso, publica = b.responderPergunta(b.pendenteDe("g", []string{"9"}), l, 1)
	if publica || !strings.Contains(aviso, "não existe") {
		t.Fatalf("esperava pergunta de criação, veio %q", aviso)
	}
	if _, publica = b.responderPergunta(b.pendenteDe("g", []string{"9"}), l, 1); !publica {
		t.Fatal("esperava publicar")
	}
	if len(l.g.Extras) != 1 || l.g.Extras[0] != "irmao da luana" || len(l.v.Confirmados) != 1 {
		t.Fatalf("extras %v confirmados %v", l.g.Extras, l.v.Confirmados)
	}
	if b.pendenteDe("g", []string{"9"}) != nil {
		t.Fatal("pergunta devia ter acabado")
	}
}

func TestVariosNomesCancelaNoSegundo(t *testing.T) {
	b, l := novaLista()
	b.iniciar("g", []string{"9"}, "!eu", "luana fulano", l)
	aviso, _ := b.responderPergunta(b.pendenteDe("g", []string{"9"}), l, 2)
	if !strings.Contains(aviso, "*fulano* não existe") {
		t.Fatalf("veio %q", aviso)
	}
	aviso, publica := b.responderPergunta(b.pendenteDe("g", []string{"9"}), l, 2)
	if publica || aviso != cancelado || len(l.v.Confirmados) != 0 || len(l.g.Extras) != 0 {
		t.Fatalf("cancelar devia não fazer nada: %q %v", aviso, l.v.Confirmados)
	}
}

func TestNomeSoOuComAspasNaoPergunta(t *testing.T) {
	b, l := novaLista()
	if _, publica := b.iniciar("g", []string{"9"}, "!eu", "luana", l); !publica || len(l.v.Confirmados) != 1 {
		t.Fatal("nome único existente devia marcar direto")
	}
	if aviso, _ := b.iniciar("g", []string{"9"}, "!add", `"irmao da luana"`, l); aviso != "" || len(l.g.Extras) != 1 {
		t.Fatalf("aspas devia adicionar direto: %q", aviso)
	}
}
