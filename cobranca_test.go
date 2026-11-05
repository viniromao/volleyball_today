package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var hojeTeste = time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)

func TestConfigurarCobranca(t *testing.T) {
	c := &Cobranca{}
	if erro := c.Configurar(`churrasco do zé 17/11 R$ 82,2 14357877733 "Vinicius Romao"`, hojeTeste); erro != "" {
		t.Fatal(erro)
	}
	if c.Nome != "churrasco do zé" || c.Dia != "17/11" || c.Valor != "82,20" || c.Pix != "14357877733" || c.Favorecido != "Vinicius Romao" {
		t.Fatalf("%+v", c)
	}
	// o que não vem fica como estava
	if erro := c.Configurar("90", hojeTeste); erro != "" || c.Valor != "90,00" || c.Pix != "14357877733" || c.Nome == "" {
		t.Fatalf("%q %+v", erro, c)
	}
	if erro := c.Configurar("82,2x", hojeTeste); erro == "" {
		t.Fatal("valor torto devia dar erro")
	}
	if erro := c.Configurar("01/01", hojeTeste); erro != "" {
		t.Fatalf("janeiro do ano que vem devia valer: %q", erro)
	}
	if erro := c.Configurar("30/02", hojeTeste); erro == "" {
		t.Fatal("data que não existe devia dar erro")
	}
}

func TestListasSeparadas(t *testing.T) {
	todos := []Membro{{IDs: []string{"1"}, Nome: "Luana"}, {IDs: []string{"2"}, Nome: "Jose"}}
	g := &Grupo{}
	v := g.Atual(hojeTeste)
	g.Pagamento = &Cobranca{}
	b := &Bot{}

	// !remove no vôlei não mexe no pagamento, e vice-versa
	lv := listaDe(g, v, todos, false)
	if aviso, _ := lv.executar("!remove", []string{"jose"}, nil); aviso != "" {
		t.Fatal(aviso)
	}
	lp := listaDe(g, v, todos, true)
	if len(lp.membros) != 2 {
		t.Fatalf("pagamento devia continuar com os 2: %v", lp.membros)
	}
	if aviso, _ := lp.executar(cmdPagRemove, []string{"luana"}, nil); aviso != "" {
		t.Fatal(aviso)
	}
	if lv = listaDe(g, v, todos, false); len(lv.membros) != 1 || lv.membros[0].Nome != "Luana" {
		t.Fatalf("vôlei devia ter só a Luana: %v", lv.membros)
	}
	if lp = listaDe(g, v, todos, true); len(lp.membros) != 1 || lp.membros[0].Nome != "Jose" {
		t.Fatalf("pagamento devia ter só o Jose: %v", lp.membros)
	}

	// pagar não vira ✅ no vôlei
	if aviso, _ := lp.executar("!paguei", []string{"jose"}, nil); aviso != "" {
		t.Fatal(aviso)
	}
	if len(v.Confirmados) != 0 {
		t.Fatalf("pagar não devia mexer no vôlei: %v", v.Confirmados)
	}
	texto := g.Pagamento.Render(lp.membros, hojeTeste)
	if !strings.Contains(texto, "💰 Jose") || strings.Contains(texto, "Luana") || !strings.Contains(texto, "Pagaram: 1") {
		t.Fatalf("pagamento:\n%s", texto)
	}
	if strings.Contains(v.Render(lv.membros, hojeTeste), "💰") {
		t.Fatal("o vôlei não deve mostrar pagamento")
	}

	// nome que não está na lista de pagamento: oferece adicionar (e só ali)
	aviso, publica := b.iniciar("g", []string{"9"}, "!paguei", "fulano", listaDe(g, v, todos, true))
	if publica || !strings.Contains(aviso, "não existe") {
		t.Fatalf("%q", aviso)
	}
	p := b.pendenteDe("g", []string{"9"})
	if !p.pag {
		t.Fatal("a pergunta devia ser da lista de pagamento")
	}
	if _, publica = b.responderPergunta(p, listaDe(g, v, todos, true), 1); !publica {
		t.Fatal("esperava publicar")
	}
	if len(g.Pagamento.Extras) != 1 || len(g.Extras) != 0 || len(g.Pagamento.Pagantes) != 2 {
		t.Fatalf("extras pag %v vôlei %v pagantes %v", g.Pagamento.Extras, g.Extras, g.Pagamento.Pagantes)
	}
}

func TestMigrarCobrancaAntiga(t *testing.T) {
	antigo := `{"g@g.us":{"votacoes":[{"dia":"05/11","data":"2026-11-05","nome":"churras","cobranca":true,"valor":"82,20","pix":"abc@x.com","favorecido":"Zé",
		"confirmados":[{"ids":["1"],"nome":"Ana","pago":true},{"ids":["2"],"nome":"Bia"}],"mensagens":[{"id":"m1","em":"2026-11-05T10:00:00Z"}],"envios":3}],
		"ocultos":[{"ids":["3"],"nome":"Caio"}],"extras":["Dani"]}}`
	var grupos map[string]*Grupo
	if err := json.Unmarshal([]byte(antigo), &grupos); err != nil {
		t.Fatal(err)
	}
	g := grupos["g@g.us"]
	if g.Elenco.Extras[0] != "Dani" || g.Votacoes[0].Envios != 3 {
		t.Fatalf("formato antigo devia continuar valendo: %+v", g)
	}
	g.migrarCobranca("2026-11-05")
	c := g.Pagamento
	if c == nil || c.Valor != "82,20" || c.Pix != "abc@x.com" || c.Nome != "churras" || len(c.Pagantes) != 1 || c.Pagantes[0].Nome != "Ana" || len(c.Extras) != 1 {
		t.Fatalf("%+v", c)
	}
	v := g.Votacoes[0]
	if v.Cobranca || v.Valor != "" || v.Confirmados[0].Pago {
		t.Fatalf("a votação devia ficar sem cobrança: %+v", v)
	}
	saida, _ := json.Marshal(g)
	if strings.Contains(string(saida), `"cobranca"`) || !strings.Contains(string(saida), `"pagamento"`) || !strings.Contains(string(saida), `"mensagens"`) {
		t.Fatalf("json: %s", saida)
	}

	// cobrança de dia que já passou some, como sumia antes
	g2 := &Grupo{Votacoes: []*Votacao{{Data: "2026-11-01", Cobranca: true, Valor: "10,00"}}}
	g2.migrarCobranca("2026-11-05")
	if g2.Pagamento != nil || g2.Votacoes[0].Cobranca {
		t.Fatal("cobrança velha devia sumir")
	}
}
