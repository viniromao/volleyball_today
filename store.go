package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func (b *Bot) arquivo() string {
	return filepath.Join(b.dir, "votacoes.json")
}

func (b *Bot) Carregar() error {
	data, err := os.ReadFile(b.arquivo())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, &b.grupos); err != nil {
		return err
	}
	hoje := b.hoje().Format(formatoData)
	for _, g := range b.grupos {
		g.migrarCobranca(hoje)
	}
	return nil
}

// migrarCobranca move a cobrança que antes ficava dentro da votação do dia pra
// lista de pagamento própria do grupo. Cobrança de dia que já passou some,
// como sumia antes.
func (g *Grupo) migrarCobranca(hoje string) {
	for _, v := range g.Votacoes {
		if !v.Cobranca && v.Valor == "" && v.Pix == "" {
			continue
		}
		if g.Pagamento == nil && v.Cobranca && v.Data >= hoje {
			c := &Cobranca{
				Nome: v.Nome, Dia: v.Dia, Data: v.Data,
				Valor: v.Valor, Pix: v.Pix, Favorecido: v.Favorecido,
				Elenco: Elenco{
					Ocultos: append([]Pessoa(nil), g.Ocultos...),
					Extras:  append([]string(nil), g.Extras...),
				},
			}
			for _, x := range v.Confirmados {
				if x.Pago {
					c.Pagantes = append(c.Pagantes, Pagante{IDs: x.IDs, Nome: x.Nome})
				}
			}
			g.Pagamento = c
		}
		v.Cobranca, v.Valor, v.Pix, v.Favorecido = false, "", "", ""
		for i := range v.Confirmados {
			v.Confirmados[i].Pago = false
		}
	}
}

func (b *Bot) Salvar() error {
	data, err := json.MarshalIndent(b.grupos, "", "  ")
	if err != nil {
		return err
	}
	tmp := b.arquivo() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, b.arquivo())
}
